package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"

	"github.com/cloudskiff/driftctl/pkg/parallel"
	"github.com/stretchr/testify/assert"

	"github.com/cloudskiff/driftctl/pkg/alerter"

	awsdeserializer "github.com/cloudskiff/driftctl/pkg/resource/aws/deserializer"

	"github.com/cloudskiff/driftctl/test/goldenfile"

	"github.com/cloudskiff/driftctl/pkg/resource"
	"github.com/cloudskiff/driftctl/pkg/terraform"
	"github.com/cloudskiff/driftctl/test"
	"github.com/cloudskiff/driftctl/test/mocks"
)

func TestS3BucketPolicySupplier_Resources(t *testing.T) {

	tests := []struct {
		test           string
		dirName        string
		bucketsIDs     []string
		bucketLocation map[string]string
		listError      error
		wantAlert      alerter.Alerts
		wantErr        bool
	}{
		{
			test:    "single bucket without policy",
			dirName: "s3_bucket_policy_no_policy",
			bucketsIDs: []string{
				"dritftctl-test-no-policy",
			},
			bucketLocation: map[string]string{
				"dritftctl-test-no-policy": "eu-west-3",
			},
			wantAlert: alerter.Alerts{},
			wantErr:   false,
		},
		{
			test: "multiple bucket with policies", dirName: "s3_bucket_policies_multiple",
			bucketsIDs: []string{
				"bucket-martin-test-drift",
				"bucket-martin-test-drift2",
				"bucket-martin-test-drift3",
			},
			bucketLocation: map[string]string{
				"bucket-martin-test-drift":  "eu-west-1",
				"bucket-martin-test-drift2": "eu-west-3",
				"bucket-martin-test-drift3": "ap-northeast-1",
			},
			wantAlert: alerter.Alerts{},
			wantErr:   false,
		},
		{
			test: "cannot list bucket", dirName: "s3_bucket_policies_list_bucket",
			bucketsIDs: nil,
			listError:  awserr.NewRequestFailure(nil, 403, ""),
			bucketLocation: map[string]string{
				"bucket-martin-test-drift":  "eu-west-1",
				"bucket-martin-test-drift2": "eu-west-3",
				"bucket-martin-test-drift3": "ap-northeast-1",
			},
			wantAlert: alerter.Alerts{"aws_s3_bucket_policy": []alerter.Alert{alerter.Alert{Message: "Ignoring aws_s3_bucket_policy from drift calculation. Listing aws_s3_bucket is forbidden.", ShouldIgnoreResource: true}}},
			wantErr:   false,
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

			factory := AwsClientFactory{config: provider.session}

			terraform.AddProvider(terraform.AWS, provider)
			resource.AddSupplier(NewS3BucketPolicySupplier(provider.Runner().SubRunner(), factory, alertr))
		}

		t.Run(tt.test, func(t *testing.T) {

			mock := mocks.NewMockAWSS3Client(tt.bucketsIDs, nil, nil, nil, tt.bucketLocation, tt.listError)
			factory := mocks.NewMockAwsClientFactory(mock)

			provider := mocks.NewMockedGoldenTFProvider(tt.dirName, terraform.Provider(terraform.AWS), shouldUpdate)
			deserializer := awsdeserializer.NewS3BucketPolicyDeserializer()
			s := &S3BucketPolicySupplier{
				provider,
				deserializer,
				factory,
				terraform.NewParallelResourceReader(parallel.NewParallelRunner(context.TODO(), 10)),
				alertr,
			}
			got, err := s.Resources()
			if (err != nil) != tt.wantErr {
				t.Errorf("Resources() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.wantAlert, alertr.Retrieve())
			test.CtyTestDiff(got, tt.dirName, provider, deserializer, shouldUpdate, t)
		})
	}
}
