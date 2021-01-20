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

func TestEC2EbsVolumeSupplier_Resources(t *testing.T) {
	tests := []struct {
		test              string
		dirName           string
		volumesPages      mocks.DescribeVolumesPagesOutput
		volumesPagesError error
		wantAlert         alerter.Alerts
		err               error
	}{
		{
			test:    "no volumes",
			dirName: "ec2_ebs_volume_empty",
			volumesPages: mocks.DescribeVolumesPagesOutput{
				{
					true,
					&ec2.DescribeVolumesOutput{},
				},
			},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:    "with volumes",
			dirName: "ec2_ebs_volume_multiple",
			volumesPages: mocks.DescribeVolumesPagesOutput{
				{
					false,
					&ec2.DescribeVolumesOutput{
						Volumes: []*ec2.Volume{
							{
								VolumeId: aws.String("vol-081c7272a57a09db1"),
							},
						},
					},
				},
				{
					true,
					&ec2.DescribeVolumesOutput{
						Volumes: []*ec2.Volume{
							{
								VolumeId: aws.String("vol-01ddc91d3d9d1318b"),
							},
						},
					},
				},
			},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:              "cannot list volumes",
			dirName:           "ec2_ebs_volume_empty",
			volumesPagesError: awserr.NewRequestFailure(nil, 403, ""),
			wantAlert:         alerter.Alerts{"aws_ebs_volume": []alerter.Alert{alerter.Alert{Message: "Ignoring aws_ebs_volume from drift calculation: Listing aws_ebs_volume is forbidden.", ShouldIgnoreResource: true}}},
			err:               nil,
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
			resource.AddSupplier(NewEC2EbsVolumeSupplier(provider.Runner(), ec2.New(provider.session), alertr))
		}

		t.Run(tt.test, func(t *testing.T) {
			provider := mocks.NewMockedGoldenTFProvider(tt.dirName, terraform.Provider(terraform.AWS), shouldUpdate)
			deserializer := awsdeserializer.NewEC2EbsVolumeDeserializer()
			client := mocks.NewMockAWSEC2EbsVolumeClient(tt.volumesPages)
			if tt.volumesPagesError != nil {
				client = mocks.NewMockAWSEC2ErrorClient(tt.volumesPagesError)
			}
			s := &EC2EbsVolumeSupplier{
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
