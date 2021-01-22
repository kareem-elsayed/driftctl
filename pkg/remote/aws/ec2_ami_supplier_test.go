package aws

import (
	"context"
	"testing"

	"github.com/cloudskiff/driftctl/pkg/parallel"
	awsdeserializer "github.com/cloudskiff/driftctl/pkg/resource/aws/deserializer"

	"github.com/cloudskiff/driftctl/test/goldenfile"

	"github.com/cloudskiff/driftctl/pkg/resource"
	"github.com/cloudskiff/driftctl/pkg/terraform"
	"github.com/cloudskiff/driftctl/test"
	"github.com/cloudskiff/driftctl/test/mocks"
)

func TestEC2AmiSupplier_Resources(t *testing.T) {
	tests := []struct {
		test    string
		dirName string
		amiIDs  []string
		err     error
	}{
		{
			test:    "no amis",
			dirName: "ec2_ami_empty",
			amiIDs:  []string{},
			err:     nil,
		},
		{
			test:    "with amis",
			dirName: "ec2_ami_multiple",
			amiIDs:  []string{"ami-03a578b46f4c3081b", "ami-025962fd8b456731f"},
			err:     nil,
		},
	}
	for _, tt := range tests {
		shouldUpdate := tt.dirName == *goldenfile.Update

		providerLibrary := terraform.NewLibrary()
		supplierLibrary := resource.NewLibrary()

		if shouldUpdate {
			provider, err := NewTerraFormProvider()
			if err != nil {
				t.Fatal(err)
			}

			providerLibrary.AddProvider(terraform.AWS, provider)
			supplierLibrary.AddSupplier(NewEC2AmiSupplier(provider))
		}

		t.Run(tt.test, func(t *testing.T) {
			provider := mocks.NewMockedGoldenTFProvider(tt.dirName, providerLibrary.Provider(terraform.AWS), shouldUpdate)
			deserializer := awsdeserializer.NewEC2AmiDeserializer()
			s := &EC2AmiSupplier{
				provider,
				deserializer,
				mocks.NewMockAWSEC2AmiClient(tt.amiIDs),
				terraform.NewParallelResourceReader(parallel.NewParallelRunner(context.TODO(), 10)),
			}
			got, err := s.Resources()
			if tt.err != err {
				t.Errorf("Expected error %+v got %+v", tt.err, err)
			}

			test.CtyTestDiff(got, tt.dirName, provider, deserializer, shouldUpdate, t)
		})
	}
}
