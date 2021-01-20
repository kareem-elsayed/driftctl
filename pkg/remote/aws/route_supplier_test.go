package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"

	"github.com/cloudskiff/driftctl/pkg/alerter"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/cloudskiff/driftctl/mocks"
	"github.com/cloudskiff/driftctl/pkg/parallel"
	"github.com/cloudskiff/driftctl/pkg/remote/deserializer"
	"github.com/cloudskiff/driftctl/pkg/resource"
	awsdeserializer "github.com/cloudskiff/driftctl/pkg/resource/aws/deserializer"
	"github.com/cloudskiff/driftctl/pkg/terraform"
	"github.com/cloudskiff/driftctl/test"
	"github.com/cloudskiff/driftctl/test/goldenfile"
	mocks2 "github.com/cloudskiff/driftctl/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRouteSupplier_Resources(t *testing.T) {
	cases := []struct {
		test      string
		dirName   string
		mocks     func(client *mocks.FakeEC2)
		wantAlert alerter.Alerts
		err       error
	}{
		{
			// route table with no route case is not possible
			// as a default route will always be present in each route table
			test:    "no route",
			dirName: "route_empty",
			mocks: func(client *mocks.FakeEC2) {
				client.On("DescribeRouteTablesPages",
					&ec2.DescribeRouteTablesInput{},
					mock.MatchedBy(func(callback func(res *ec2.DescribeRouteTablesOutput, lastPage bool) bool) bool {
						callback(&ec2.DescribeRouteTablesOutput{}, true)
						return true
					})).Return(nil)
			},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:    "mixed default_route_table and route_table",
			dirName: "route",
			mocks: func(client *mocks.FakeEC2) {
				client.On("DescribeRouteTablesPages",
					&ec2.DescribeRouteTablesInput{},
					mock.MatchedBy(func(callback func(res *ec2.DescribeRouteTablesOutput, lastPage bool) bool) bool {
						callback(&ec2.DescribeRouteTablesOutput{
							RouteTables: []*ec2.RouteTable{
								{
									RouteTableId: awssdk.String("rtb-096bdfb69309c54c3"), // table1
									Routes: []*ec2.Route{
										{
											DestinationCidrBlock: awssdk.String("10.0.0.0/16"),
											Origin:               awssdk.String("CreateRouteTable"), // default route
										},
										{
											DestinationCidrBlock: awssdk.String("1.1.1.1/32"),
											GatewayId:            awssdk.String("igw-030e74f73bd67f21b"),
										},
										{
											DestinationIpv6CidrBlock: awssdk.String("::/0"),
											GatewayId:                awssdk.String("igw-030e74f73bd67f21b"),
										},
									},
								},
								{
									RouteTableId: awssdk.String("rtb-0169b0937fd963ddc"), // table2
									Routes: []*ec2.Route{
										{
											DestinationCidrBlock: awssdk.String("10.0.0.0/16"),
											Origin:               awssdk.String("CreateRouteTable"), // default route
										},
										{
											DestinationCidrBlock: awssdk.String("0.0.0.0/0"),
											GatewayId:            awssdk.String("igw-030e74f73bd67f21b"),
										},
										{
											DestinationIpv6CidrBlock: awssdk.String("::/0"),
											GatewayId:                awssdk.String("igw-030e74f73bd67f21b"),
										},
									},
								},
							},
						}, false)
						callback(&ec2.DescribeRouteTablesOutput{
							RouteTables: []*ec2.RouteTable{
								{
									RouteTableId: awssdk.String("rtb-02780c485f0be93c5"), // default_table
									VpcId:        awssdk.String("vpc-09fe5abc2309ba49d"),
									Associations: []*ec2.RouteTableAssociation{
										{
											Main: awssdk.Bool(true),
										},
									},
									Routes: []*ec2.Route{
										{
											DestinationCidrBlock: awssdk.String("10.0.0.0/16"),
											Origin:               awssdk.String("CreateRouteTable"), // default route
										},
										{
											DestinationCidrBlock: awssdk.String("10.1.1.0/24"),
											GatewayId:            awssdk.String("igw-030e74f73bd67f21b"),
										},
										{
											DestinationCidrBlock: awssdk.String("10.1.2.0/24"),
											GatewayId:            awssdk.String("igw-030e74f73bd67f21b"),
										},
									},
								},
								{
									RouteTableId: awssdk.String(""), // table3
									Routes: []*ec2.Route{
										{
											DestinationCidrBlock: awssdk.String("10.0.0.0/16"),
											Origin:               awssdk.String("CreateRouteTable"), // default route
										},
									},
								},
							},
						}, true)
						return true
					})).Return(nil)
			},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:    "cannot list route table",
			dirName: "route_empty",
			mocks: func(client *mocks.FakeEC2) {
				client.On("DescribeRouteTablesPages",
					&ec2.DescribeRouteTablesInput{},
					mock.MatchedBy(func(callback func(res *ec2.DescribeRouteTablesOutput, lastPage bool) bool) bool {
						return true
					})).Return(awserr.NewRequestFailure(nil, 403, ""))
			},
			wantAlert: alerter.Alerts{"aws_route": []alerter.Alert{alerter.Alert{Message: "Ignoring aws_route from drift calculation. Listing aws_route_table is forbidden.", ShouldIgnoreResource: true}}},
			err:       nil,
		},
	}
	for _, c := range cases {
		alertr := alerter.NewAlerter()

		shouldUpdate := c.dirName == *goldenfile.Update
		if shouldUpdate {
			provider, err := NewTerraFormProvider()
			if err != nil {
				t.Fatal(err)
			}

			terraform.AddProvider(terraform.AWS, provider)
			resource.AddSupplier(NewRouteSupplier(provider.Runner(), ec2.New(provider.session), alertr))
		}

		t.Run(c.test, func(tt *testing.T) {
			fakeEC2 := mocks.FakeEC2{}
			c.mocks(&fakeEC2)
			provider := mocks2.NewMockedGoldenTFProvider(c.dirName, terraform.Provider(terraform.AWS), shouldUpdate)
			routeDeserializer := awsdeserializer.NewRouteDeserializer()
			s := &RouteSupplier{
				provider,
				routeDeserializer,
				&fakeEC2,
				terraform.NewParallelResourceReader(parallel.NewParallelRunner(context.TODO(), 10)),
				alertr,
			}
			got, err := s.Resources()
			if c.err != err {
				tt.Errorf("Expected error %+v got %+v", c.err, err)
			}

			mock.AssertExpectationsForObjects(tt)
			deserializers := []deserializer.CTYDeserializer{routeDeserializer}
			assert.Equal(t, c.wantAlert, alertr.Retrieve())
			test.CtyTestDiffMixed(got, c.dirName, provider, deserializers, shouldUpdate, tt)
		})
	}
}
