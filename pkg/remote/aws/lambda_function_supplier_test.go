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
	"github.com/aws/aws-sdk-go/service/lambda"
)

func TestLambdaFunctionSupplier_Resources(t *testing.T) {
	tests := []struct {
		test           string
		dirName        string
		functionsPages mocks.ListFunctionsPagesOutput
		listError      error
		wantAlert      alerter.Alerts
		err            error
	}{
		{
			test:    "no lambda functions",
			dirName: "lambda_function_empty",
			functionsPages: mocks.ListFunctionsPagesOutput{
				{
					true,
					&lambda.ListFunctionsOutput{},
				},
			},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:    "with lambda functions",
			dirName: "lambda_function_multiple",
			functionsPages: mocks.ListFunctionsPagesOutput{
				{
					false,
					&lambda.ListFunctionsOutput{
						Functions: []*lambda.FunctionConfiguration{
							{
								FunctionName: aws.String("foo"),
							},
						},
					},
				},
				{
					true,
					&lambda.ListFunctionsOutput{
						Functions: []*lambda.FunctionConfiguration{
							{
								FunctionName: aws.String("bar"),
							},
						},
					},
				},
			},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:    "One lambda with signing",
			dirName: "lambda_function_signed",
			functionsPages: mocks.ListFunctionsPagesOutput{
				{
					false,
					&lambda.ListFunctionsOutput{
						Functions: []*lambda.FunctionConfiguration{
							{
								FunctionName: aws.String("foo"),
							},
						},
					},
				},
			},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:      "cannot list lambda functions",
			dirName:   "lambda_function_empty",
			listError: awserr.NewRequestFailure(nil, 403, ""),
			wantAlert: alerter.Alerts{"aws_lambda_function": []alerter.Alert{alerter.Alert{Message: "Ignoring aws_lambda_function from drift calculation: Listing aws_lambda_function is forbidden.", ShouldIgnoreResource: true}}},
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
			resource.AddSupplier(NewLambdaFunctionSupplier(provider.Runner(), lambda.New(provider.session), alertr))
		}

		t.Run(tt.test, func(t *testing.T) {
			provider := mocks.NewMockedGoldenTFProvider(tt.dirName, terraform.Provider(terraform.AWS), shouldUpdate)
			deserializer := awsdeserializer.NewLambdaFunctionDeserializer()
			client := mocks.NewMockAWSLambdaClient(tt.functionsPages)
			if tt.listError != nil {
				client = mocks.NewMockAWSLambdaErrorClient(tt.listError)
			}
			s := &LambdaFunctionSupplier{
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
