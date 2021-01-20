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
)

func TestS3BucketMetricSupplier_Resources(t *testing.T) {

	tests := []struct {
		test           string
		dirName        string
		bucketsIDs     []string
		bucketLocation map[string]string
		metricsIDs     map[string][]string
		listError      error
		wantAlert      alerter.Alerts
		wantErr        bool
	}{
		{
			test: "multiple bucket with multiple metrics", dirName: "s3_bucket_metrics_multiple",
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
			metricsIDs: map[string][]string{
				"bucket-martin-test-drift": {
					"Metrics_Bucket1",
					"Metrics2_Bucket1",
				},
				"bucket-martin-test-drift2": {
					"Metrics_Bucket2",
					"Metrics2_Bucket2",
				},
				"bucket-martin-test-drift3": {
					"Metrics_Bucket3",
					"Metrics2_Bucket3",
				},
			},
			wantAlert: alerter.Alerts{},
			wantErr:   false,
		},
		{
			test: "cannot list bucket", dirName: "s3_bucket_metrics_list_bucket",
			bucketsIDs: nil,
			listError:  awserr.NewRequestFailure(nil, 403, ""),
			bucketLocation: map[string]string{
				"bucket-martin-test-drift":  "eu-west-1",
				"bucket-martin-test-drift2": "eu-west-3",
				"bucket-martin-test-drift3": "ap-northeast-1",
			},
			metricsIDs: map[string][]string{
				"bucket-martin-test-drift": {
					"Metrics_Bucket1",
					"Metrics2_Bucket1",
				},
				"bucket-martin-test-drift2": {
					"Metrics_Bucket2",
					"Metrics2_Bucket2",
				},
				"bucket-martin-test-drift3": {
					"Metrics_Bucket3",
					"Metrics2_Bucket3",
				},
			},
			wantAlert: alerter.Alerts{"aws_s3_bucket_metric": []alerter.Alert{alerter.Alert{Message: "Ignoring aws_s3_bucket_metric from drift calculation. Listing aws_s3_bucket is forbidden.", ShouldIgnoreResource: true}}},
			wantErr:   false,
		},
		{
			test: "cannot list metrics", dirName: "s3_bucket_metrics_list_metrics",
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
			metricsIDs: nil,
			listError:  awserr.NewRequestFailure(nil, 403, ""),
			wantAlert:  alerter.Alerts{"aws_s3_bucket_metric": []alerter.Alert{alerter.Alert{Message: "Ignoring aws_s3_bucket_metric from drift calculation: Listing aws_s3_bucket_metric is forbidden.", ShouldIgnoreResource: true}}},
			wantErr:    false,
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
			resource.AddSupplier(NewS3BucketMetricSupplier(provider.Runner().SubRunner(), factory, alertr))
		}

		t.Run(tt.test, func(t *testing.T) {

			mock := mocks.NewMockAWSS3Client(tt.bucketsIDs, nil, nil, tt.metricsIDs, tt.bucketLocation, tt.listError)
			factory := mocks.NewMockAwsClientFactory(mock)

			provider := mocks.NewMockedGoldenTFProvider(tt.dirName, terraform.Provider(terraform.AWS), shouldUpdate)
			deserializer := awsdeserializer.NewS3BucketMetricDeserializer()
			s := &S3BucketMetricSupplier{
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
