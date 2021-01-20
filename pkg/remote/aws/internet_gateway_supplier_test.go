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

func TestInternetGatewaySupplier_Resources(t *testing.T) {
	cases := []struct {
		test      string
		dirName   string
		mocks     func(client *mocks.FakeEC2)
		wantAlert alerter.Alerts
		err       error
	}{
		{
			test:    "no internet gateways",
			dirName: "internet_gateway_empty",
			mocks: func(client *mocks.FakeEC2) {
				client.On("DescribeInternetGatewaysPages",
					&ec2.DescribeInternetGatewaysInput{},
					mock.MatchedBy(func(callback func(res *ec2.DescribeInternetGatewaysOutput, lastPage bool) bool) bool {
						callback(&ec2.DescribeInternetGatewaysOutput{}, true)
						return true
					})).Return(nil)
			},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:    "multiple internet gateways",
			dirName: "internet_gateway_multiple",
			mocks: func(client *mocks.FakeEC2) {
				client.On("DescribeInternetGatewaysPages",
					&ec2.DescribeInternetGatewaysInput{},
					mock.MatchedBy(func(callback func(res *ec2.DescribeInternetGatewaysOutput, lastPage bool) bool) bool {
						callback(&ec2.DescribeInternetGatewaysOutput{
							InternetGateways: []*ec2.InternetGateway{
								{
									InternetGatewayId: awssdk.String("igw-0184eb41aadc62d1c"),
								},
								{
									InternetGatewayId: awssdk.String("igw-047b487f5c60fca99"),
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
			test:    "cannot list internet gateways",
			dirName: "internet_gateway_empty",
			mocks: func(client *mocks.FakeEC2) {
				client.On("DescribeInternetGatewaysPages",
					&ec2.DescribeInternetGatewaysInput{},
					mock.MatchedBy(func(callback func(res *ec2.DescribeInternetGatewaysOutput, lastPage bool) bool) bool {
						return true
					})).Return(awserr.NewRequestFailure(nil, 403, ""))
			},
			wantAlert: alerter.Alerts{"aws_internet_gateway": []alerter.Alert{alerter.Alert{Message: "Ignoring aws_internet_gateway from drift calculation: Listing aws_internet_gateway is forbidden.", ShouldIgnoreResource: true}}},
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
			resource.AddSupplier(NewInternetGatewaySupplier(provider.Runner(), ec2.New(provider.session), alertr))
		}

		t.Run(c.test, func(tt *testing.T) {
			fakeEC2 := mocks.FakeEC2{}
			c.mocks(&fakeEC2)
			provider := mocks2.NewMockedGoldenTFProvider(c.dirName, terraform.Provider(terraform.AWS), shouldUpdate)
			internetGatewayDeserializer := awsdeserializer.NewInternetGatewayDeserializer()
			s := &InternetGatewaySupplier{
				provider,
				internetGatewayDeserializer,
				&fakeEC2,
				terraform.NewParallelResourceReader(parallel.NewParallelRunner(context.TODO(), 10)),
				alertr,
			}
			got, err := s.Resources()
			if c.err != err {
				tt.Errorf("Expected error %+v got %+v", c.err, err)
			}

			mock.AssertExpectationsForObjects(tt)
			deserializers := []deserializer.CTYDeserializer{internetGatewayDeserializer}
			assert.Equal(t, c.wantAlert, alertr.Retrieve())
			test.CtyTestDiffMixed(got, c.dirName, provider, deserializers, shouldUpdate, tt)
		})
	}
}
