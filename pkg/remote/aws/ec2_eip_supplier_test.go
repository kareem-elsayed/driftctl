package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/stretchr/testify/assert"

	"github.com/cloudskiff/driftctl/pkg/alerter"
	"github.com/cloudskiff/driftctl/pkg/parallel"

	awsdeserializer "github.com/cloudskiff/driftctl/pkg/resource/aws/deserializer"

	"github.com/cloudskiff/driftctl/test/goldenfile"

	"github.com/cloudskiff/driftctl/pkg/resource"
	"github.com/cloudskiff/driftctl/pkg/terraform"
	"github.com/cloudskiff/driftctl/test"
	"github.com/cloudskiff/driftctl/test/mocks"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func TestEC2EipSupplier_Resources(t *testing.T) {
	tests := []struct {
		test      string
		dirName   string
		addresses []*ec2.Address
		listError error
		wantAlert alerter.Alerts
		err       error
	}{
		{
			test:      "no eips",
			dirName:   "ec2_eip_empty",
			addresses: []*ec2.Address{},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:    "with eips",
			dirName: "ec2_eip_multiple",
			addresses: []*ec2.Address{
				{
					AllocationId: aws.String("eipalloc-017d5267e4dda73f1"),
				},
				{
					AllocationId: aws.String("eipalloc-0cf714dc097c992cc"),
				},
			},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:      "Cannot list eips",
			dirName:   "ec2_eip_empty",
			listError: awserr.NewRequestFailure(nil, 403, ""),
			wantAlert: alerter.Alerts{"aws_eip": []alerter.Alert{alerter.Alert{Message: "Ignoring aws_eip from drift calculation: Listing aws_eip is forbidden.", ShouldIgnoreResource: true}}},
			err:       nil,
		},
	}
	for _, tt := range tests {
		alertr := alerter.NewAlerter()

		shouldUpdate := tt.dirName == *goldenfile.Update
		if shouldUpdate {
			provider, err := NewTerraFormProvider()
			if err != nil {
				t.Fatal(err)
			}

			terraform.AddProvider(terraform.AWS, provider)
			resource.AddSupplier(NewEC2EipSupplier(provider.Runner(), ec2.New(provider.session), alertr))
		}

		t.Run(tt.test, func(t *testing.T) {
			provider := mocks.NewMockedGoldenTFProvider(tt.dirName, terraform.Provider(terraform.AWS), shouldUpdate)
			deserializer := awsdeserializer.NewEC2EipDeserializer()
			client := mocks.NewMockAWSEC2EipClient(tt.addresses)
			if tt.listError != nil {
				client = mocks.NewMockAWSEC2ErrorClient(tt.listError)
			}
			s := &EC2EipSupplier{
				provider,
				deserializer,
				client,
				terraform.NewParallelResourceReader(parallel.NewParallelRunner(context.TODO(), 10)),
				alertr,
			}
			got, err := s.Resources()
			if tt.err != err {
				t.Errorf("Expected error %+v got %+v", tt.err, err)
			}

			assert.Equal(t, tt.wantAlert, alertr.Retrieve())
			test.CtyTestDiff(got, tt.dirName, provider, deserializer, shouldUpdate, t)
		})
	}
}
