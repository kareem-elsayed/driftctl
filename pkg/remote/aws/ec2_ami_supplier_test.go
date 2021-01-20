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

	"github.com/aws/aws-sdk-go/service/ec2"
)

func TestEC2AmiSupplier_Resources(t *testing.T) {
	tests := []struct {
		test      string
		dirName   string
		amiIDs    []string
		listError error
		wantAlert alerter.Alerts
		err       error
	}{
		{
			test:      "no amis",
			dirName:   "ec2_ami_empty",
			amiIDs:    []string{},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:      "with amis",
			dirName:   "ec2_ami_multiple",
			amiIDs:    []string{"ami-03a578b46f4c3081b", "ami-025962fd8b456731f"},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:      "cannot list amis",
			dirName:   "ec2_ami_empty",
			listError: awserr.NewRequestFailure(nil, 403, ""),
			wantAlert: alerter.Alerts{"aws_ami": []alerter.Alert{alerter.Alert{Message: "Ignoring aws_ami from drift calculation: Listing aws_ami is forbidden.", ShouldIgnoreResource: true}}},
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
			resource.AddSupplier(NewEC2AmiSupplier(provider.Runner(), ec2.New(provider.session), alertr))
		}

		t.Run(tt.test, func(t *testing.T) {
			provider := mocks.NewMockedGoldenTFProvider(tt.dirName, terraform.Provider(terraform.AWS), shouldUpdate)
			deserializer := awsdeserializer.NewEC2AmiDeserializer()
			client := mocks.NewMockAWSEC2AmiClient(tt.amiIDs)
			if tt.listError != nil {
				client = mocks.NewMockAWSEC2ErrorClient(tt.listError)
			}
			s := &EC2AmiSupplier{
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
