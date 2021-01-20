package aws

import (
	"reflect"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/cloudskiff/driftctl/pkg/alerter"
	"github.com/cloudskiff/driftctl/pkg/parallel"
	"github.com/sirupsen/logrus"

	"github.com/cloudskiff/driftctl/pkg/remote/deserializer"
	"github.com/cloudskiff/driftctl/pkg/resource"
	"github.com/cloudskiff/driftctl/pkg/resource/aws"
	awsdeserializer "github.com/cloudskiff/driftctl/pkg/resource/aws/deserializer"
	"github.com/cloudskiff/driftctl/pkg/terraform"
	"github.com/zclconf/go-cty/cty"
)

type S3BucketNotificationSupplier struct {
	reader       terraform.ResourceReader
	deserializer deserializer.CTYDeserializer
	factory      AwsClientFactoryInterface
	runner       *terraform.ParallelResourceReader
	alerter      *alerter.Alerter
}

func NewS3BucketNotificationSupplier(runner *parallel.ParallelRunner, factory AwsClientFactoryInterface, alerter *alerter.Alerter) *S3BucketNotificationSupplier {
	return &S3BucketNotificationSupplier{
		terraform.Provider(terraform.AWS),
		awsdeserializer.NewS3BucketNotificationDeserializer(),
		factory,
		terraform.NewParallelResourceReader(runner),
		alerter,
	}
}

func (s *S3BucketNotificationSupplier) Resources() ([]resource.Resource, error) {
	input := &s3.ListBucketsInput{}

	client := s.factory.GetS3Client(nil)
	response, err := client.ListBuckets(input)
	if err != nil {
		handled := handleListAwsErrorWithMessage(err, aws.AwsS3BucketNotificationResourceType, s.alerter, aws.AwsS3BucketResourceType)
		if !handled {
			return nil, err
		}
		return []resource.Resource{}, nil
	}

	for _, bucket := range response.Buckets {
		name := *bucket.Name
		region, err := readBucketRegion(&client, name)
		if err != nil {
			return nil, err
		}
		if region == "" {
			continue
		}
		s.listBucketNotificationConfiguration(*bucket.Name, region)
	}
	ctyVals, err := s.runner.Wait()
	if err != nil {
		return nil, err
	}
	deserializedValues, err := s.deserializer.Deserialize(ctyVals)
	results := make([]resource.Resource, 0, len(deserializedValues))
	if err != nil {
		return deserializedValues, err
	}
	for _, val := range deserializedValues {
		res, ok := val.(*aws.AwsS3BucketNotification)
		if ok {
			if (res.LambdaFunction != nil && len(*res.LambdaFunction) > 0) ||
				(res.Queue != nil && len(*res.Queue) > 0) ||
				(res.Topic != nil && len(*res.Topic) > 0) {
				results = append(results, res)
			}
		}
	}
	return results, nil
}

func (s *S3BucketNotificationSupplier) listBucketNotificationConfiguration(name, region string) {
	s.runner.Run(func() (cty.Value, error) {
		s3BucketPolicy, err := s.reader.ReadResource(terraform.ReadResourceArgs{
			Ty: aws.AwsS3BucketNotificationResourceType,
			ID: name,
			Attributes: map[string]string{
				"aws_region": region,
			},
		})
		if err != nil {
			logrus.Errorf("ERROOORRR %s", reflect.TypeOf(err))

			return cty.NilVal, err
		}
		return *s3BucketPolicy, err
	})
}
