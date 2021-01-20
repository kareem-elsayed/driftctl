package aws

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/aws/aws-sdk-go/aws/awserr"

	"github.com/cloudskiff/driftctl/pkg/alerter"
	"github.com/cloudskiff/driftctl/pkg/parallel"

	awsdeserializer "github.com/cloudskiff/driftctl/pkg/resource/aws/deserializer"

	"github.com/cloudskiff/driftctl/test/goldenfile"

	"github.com/cloudskiff/driftctl/pkg/resource"
	"github.com/cloudskiff/driftctl/pkg/terraform"
	"github.com/cloudskiff/driftctl/test"
	"github.com/cloudskiff/driftctl/test/mocks"

	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
)

func TestDBInstanceSupplier_Resources(t *testing.T) {

	tests := []struct {
		test                string
		dirName             string
		instancesPages      mocks.DescribeDBInstancesPagesOutput
		instancesPagesError error
		wantAlert           alerter.Alerts
		err                 error
	}{
		{
			test:    "no dbs",
			dirName: "db_instance_empty",
			instancesPages: mocks.DescribeDBInstancesPagesOutput{
				{
					true,
					&rds.DescribeDBInstancesOutput{},
				},
			},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:    "single db",
			dirName: "db_instance_single",
			instancesPages: mocks.DescribeDBInstancesPagesOutput{
				{
					true,
					&rds.DescribeDBInstancesOutput{
						DBInstances: []*rds.DBInstance{
							{
								DBInstanceIdentifier: awssdk.String("terraform-20201015115018309600000001"),
							},
						},
					},
				},
			},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:    "multiples mixed db",
			dirName: "db_instance_multiple",
			instancesPages: mocks.DescribeDBInstancesPagesOutput{
				{
					true,
					&rds.DescribeDBInstancesOutput{
						DBInstances: []*rds.DBInstance{
							{
								DBInstanceIdentifier: awssdk.String("terraform-20201015115018309600000001"),
							},
							{
								DBInstanceIdentifier: awssdk.String("database-1"),
							},
						},
					},
				},
			},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:    "multiples mixed db",
			dirName: "db_instance_multiple",
			instancesPages: mocks.DescribeDBInstancesPagesOutput{
				{
					true,
					&rds.DescribeDBInstancesOutput{
						DBInstances: []*rds.DBInstance{
							{
								DBInstanceIdentifier: awssdk.String("terraform-20201015115018309600000001"),
							},
							{
								DBInstanceIdentifier: awssdk.String("database-1"),
							},
						},
					},
				},
			},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:                "Cannot list db instances",
			dirName:             "db_instance_empty",
			instancesPagesError: awserr.NewRequestFailure(nil, 403, ""),
			wantAlert: alerter.Alerts{
				"aws_db_instance": []alerter.Alert{alerter.Alert{Message: "Ignoring aws_db_instance from drift calculation: Listing aws_db_instance is forbidden.", ShouldIgnoreResource: true}}},
			err: nil,
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
			resource.AddSupplier(NewDBInstanceSupplier(provider.Runner(), rds.New(provider.session), alertr))
		}

		t.Run(tt.test, func(t *testing.T) {
			provider := mocks.NewMockedGoldenTFProvider(tt.dirName, terraform.Provider(terraform.AWS), shouldUpdate)
			deserializer := awsdeserializer.NewDBInstanceDeserializer()

			client := mocks.NewMockAWSRDSClient(tt.instancesPages)
			if tt.instancesPagesError != nil {
				client = mocks.NewMockAWSRDSErrorClient(tt.instancesPagesError)
			}

			s := &DBInstanceSupplier{
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
