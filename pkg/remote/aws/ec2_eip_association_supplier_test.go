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

func TestEC2EipAssociationSupplier_Resources(t *testing.T) {
	tests := []struct {
		test      string
		dirName   string
		addresses []*ec2.Address
		listError error
		wantAlert alerter.Alerts
		err       error
	}{
		{
			test:      "no eip associations",
			dirName:   "ec2_eip_association_empty",
			addresses: []*ec2.Address{},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:    "with eip associations",
			dirName: "ec2_eip_association_single",
			addresses: []*ec2.Address{
				{
					AssociationId: aws.String("eipassoc-0e9a7356e30f0c3d1"),
				},
			},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:      "Cannot list eip associations",
			dirName:   "ec2_eip_association_empty",
			listError: awserr.NewRequestFailure(nil, 403, ""),
			wantAlert: alerter.Alerts{"aws_eip_association": []alerter.Alert{alerter.Alert{Message: "Ignoring aws_eip_association from drift calculation: Listing aws_eip_association is forbidden.", ShouldIgnoreResource: true}}},
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
			resource.AddSupplier(NewEC2EipAssociationSupplier(provider.Runner(), ec2.New(provider.session), alertr))
		}

		t.Run(tt.test, func(t *testing.T) {
			provider := mocks.NewMockedGoldenTFProvider(tt.dirName, terraform.Provider(terraform.AWS), shouldUpdate)
			deserializer := awsdeserializer.NewEC2EipAssociationDeserializer()
			client := mocks.NewMockAWSEC2EipClient(tt.addresses)
			if tt.listError != nil {
				client = mocks.NewMockAWSEC2ErrorClient(tt.listError)
			}
			s := &EC2EipAssociationSupplier{
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
