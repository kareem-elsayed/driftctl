package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/cloudskiff/driftctl/mocks"
	"github.com/cloudskiff/driftctl/pkg/alerter"
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

func TestNatGatewaySupplier_Resources(t *testing.T) {
	cases := []struct {
		test      string
		dirName   string
		mocks     func(client *mocks.FakeEC2)
		wantAlert alerter.Alerts
		err       error
	}{
		{
			test:    "no gateway",
			dirName: "nat_gateway_empty",
			mocks: func(client *mocks.FakeEC2) {
				client.On("DescribeNatGatewaysPages",
					&ec2.DescribeNatGatewaysInput{},
					mock.MatchedBy(func(callback func(res *ec2.DescribeNatGatewaysOutput, lastPage bool) bool) bool {
						callback(&ec2.DescribeNatGatewaysOutput{}, true)
						return true
					})).Return(nil)
			},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:    "single aws_nat_gateway",
			dirName: "nat_gateway",
			mocks: func(client *mocks.FakeEC2) {
				client.On("DescribeNatGatewaysPages",
					&ec2.DescribeNatGatewaysInput{},
					mock.MatchedBy(func(callback func(res *ec2.DescribeNatGatewaysOutput, lastPage bool) bool) bool {
						callback(&ec2.DescribeNatGatewaysOutput{
							NatGateways: []*ec2.NatGateway{
								{
									NatGatewayId: aws.String("nat-0a5408508b19ef490"), // nat1
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
			test:    "cannot list gateway",
			dirName: "nat_gateway_empty",
			mocks: func(client *mocks.FakeEC2) {
				client.On("DescribeNatGatewaysPages",
					&ec2.DescribeNatGatewaysInput{},
					mock.MatchedBy(func(callback func(res *ec2.DescribeNatGatewaysOutput, lastPage bool) bool) bool {
						return true
					})).Return(awserr.NewRequestFailure(nil, 403, ""))
			},
			wantAlert: alerter.Alerts{"aws_nat_gateway": []alerter.Alert{alerter.Alert{Message: "Ignoring aws_nat_gateway from drift calculation: Listing aws_nat_gateway is forbidden.", ShouldIgnoreResource: true}}},
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
			resource.AddSupplier(NewNatGatewaySupplier(provider.Runner(), ec2.New(provider.session), alertr))
		}

		t.Run(c.test, func(tt *testing.T) {
			fakeEC2 := mocks.FakeEC2{}
			c.mocks(&fakeEC2)
			provider := mocks2.NewMockedGoldenTFProvider(c.dirName, terraform.Provider(terraform.AWS), shouldUpdate)
			natGatewaydeserializer := awsdeserializer.NewNatGatewayDeserializer()
			s := &NatGatewaySupplier{
				provider,
				natGatewaydeserializer,
				&fakeEC2,
				terraform.NewParallelResourceReader(parallel.NewParallelRunner(context.TODO(), 10)),
				alertr,
			}
			got, err := s.Resources()
			if c.err != err {
				tt.Errorf("Expected error %+v got %+v", c.err, err)
			}

			mock.AssertExpectationsForObjects(tt)
			deserializers := []deserializer.CTYDeserializer{natGatewaydeserializer}
			assert.Equal(t, c.wantAlert, alertr.Retrieve())
			test.CtyTestDiffMixed(got, c.dirName, provider, deserializers, shouldUpdate, tt)
		})
	}
}
