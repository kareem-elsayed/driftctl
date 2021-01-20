package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"

	"github.com/cloudskiff/driftctl/pkg/alerter"

	"github.com/aws/aws-sdk-go/aws"
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

func TestRouteTableSupplier_Resources(t *testing.T) {
	cases := []struct {
		test      string
		dirName   string
		mocks     func(client *mocks.FakeEC2)
		wantAlert alerter.Alerts
		err       error
	}{
		{
			test:    "no route table",
			dirName: "route_table_empty",
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
			dirName: "route_table",
			mocks: func(client *mocks.FakeEC2) {
				client.On("DescribeRouteTablesPages",
					&ec2.DescribeRouteTablesInput{},
					mock.MatchedBy(func(callback func(res *ec2.DescribeRouteTablesOutput, lastPage bool) bool) bool {
						callback(&ec2.DescribeRouteTablesOutput{
							RouteTables: []*ec2.RouteTable{
								{
									RouteTableId: aws.String("rtb-08b7b71af15e183ce"), // table1
								},
								{
									RouteTableId: aws.String("rtb-0002ac731f6fdea55"), // table2
								},
							},
						}, false)
						callback(&ec2.DescribeRouteTablesOutput{
							RouteTables: []*ec2.RouteTable{
								{
									RouteTableId: aws.String("rtb-0eabf071c709c0976"), // default_table
									VpcId:        awssdk.String("vpc-0b4a6b3536da20ecd"),
									Associations: []*ec2.RouteTableAssociation{
										{
											Main: awssdk.Bool(true),
										},
									},
								},
								{
									RouteTableId: aws.String("rtb-0c55d55593f33fbac"), // table3
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
			dirName: "route_table_empty",
			mocks: func(client *mocks.FakeEC2) {
				client.On("DescribeRouteTablesPages",
					&ec2.DescribeRouteTablesInput{},
					mock.MatchedBy(func(callback func(res *ec2.DescribeRouteTablesOutput, lastPage bool) bool) bool {
						callback(&ec2.DescribeRouteTablesOutput{}, true)
						return true
					})).Return(awserr.NewRequestFailure(nil, 403, ""))
			},
			wantAlert: alerter.Alerts{"aws_route_table": []alerter.Alert{alerter.Alert{Message: "Ignoring aws_route_table from drift calculation: Listing aws_route_table is forbidden.", ShouldIgnoreResource: true}}},
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
			resource.AddSupplier(NewRouteTableSupplier(provider.Runner(), ec2.New(provider.session), alertr))
		}

		t.Run(c.test, func(tt *testing.T) {
			fakeEC2 := mocks.FakeEC2{}
			c.mocks(&fakeEC2)
			provider := mocks2.NewMockedGoldenTFProvider(c.dirName, terraform.Provider(terraform.AWS), shouldUpdate)
			routeTableDeserializer := awsdeserializer.NewRouteTableDeserializer()
			defaultRouteTableDeserializer := awsdeserializer.NewDefaultRouteTableDeserializer()
			s := &RouteTableSupplier{
				provider,
				defaultRouteTableDeserializer,
				routeTableDeserializer,
				&fakeEC2,
				terraform.NewParallelResourceReader(parallel.NewParallelRunner(context.TODO(), 10)),
				terraform.NewParallelResourceReader(parallel.NewParallelRunner(context.TODO(), 10)),
				alertr,
			}
			got, err := s.Resources()
			if c.err != err {
				tt.Errorf("Expected error %+v got %+v", c.err, err)
			}

			mock.AssertExpectationsForObjects(tt)
			deserializers := []deserializer.CTYDeserializer{routeTableDeserializer, defaultRouteTableDeserializer}
			assert.Equal(t, c.wantAlert, alertr.Retrieve())
			test.CtyTestDiffMixed(got, c.dirName, provider, deserializers, shouldUpdate, tt)
		})
	}
}
