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

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/service/rds"

	"github.com/cloudskiff/driftctl/pkg/resource"
	"github.com/cloudskiff/driftctl/pkg/terraform"
	"github.com/cloudskiff/driftctl/test"
	"github.com/cloudskiff/driftctl/test/mocks"
)

func TestDBSubnetGroupSupplier_Resources(t *testing.T) {

	tests := []struct {
		test             string
		dirName          string
		subnets          mocks.DescribeSubnetGroupResponse
		subnetsListError error
		wantAlert        alerter.Alerts
		err              error
	}{
		{
			test:    "no subnets",
			dirName: "db_subnet_empty",
			subnets: mocks.DescribeSubnetGroupResponse{
				{
					true,
					&rds.DescribeDBSubnetGroupsOutput{},
				},
			},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:    "multiples db subnets",
			dirName: "db_subnet_multiples",
			subnets: mocks.DescribeSubnetGroupResponse{
				{
					false,
					&rds.DescribeDBSubnetGroupsOutput{
						DBSubnetGroups: []*rds.DBSubnetGroup{
							&rds.DBSubnetGroup{
								DBSubnetGroupName: aws.String("foo"),
							},
						},
					},
				},
				{
					true,
					&rds.DescribeDBSubnetGroupsOutput{
						DBSubnetGroups: []*rds.DBSubnetGroup{
							&rds.DBSubnetGroup{
								DBSubnetGroupName: aws.String("bar"),
							},
						},
					},
				},
			},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:             "Cannot list subnet",
			dirName:          "db_subnet_empty",
			subnetsListError: awserr.NewRequestFailure(nil, 403, ""),
			wantAlert:        alerter.Alerts{"aws_db_subnet_group": []alerter.Alert{alerter.Alert{Message: "Ignoring aws_db_subnet_group from drift calculation: Listing aws_db_subnet_group is forbidden.", ShouldIgnoreResource: true}}},
			err:              nil,
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
			deserializer := awsdeserializer.NewDBSubnetGroupDeserializer()
			client := mocks.NewMockAWSRDSSubnetGroupClient(tt.subnets)
			if tt.subnetsListError != nil {
				client = mocks.NewMockAWSRDSErrorClient(tt.subnetsListError)
			}
			s := &DBSubnetGroupSupplier{
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
