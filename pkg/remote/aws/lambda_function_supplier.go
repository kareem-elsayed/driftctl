package aws

import (
	"github.com/cloudskiff/driftctl/pkg/alerter"
	"github.com/cloudskiff/driftctl/pkg/parallel"

	"github.com/cloudskiff/driftctl/pkg/remote/deserializer"
	"github.com/cloudskiff/driftctl/pkg/resource"
	resourceaws "github.com/cloudskiff/driftctl/pkg/resource/aws"
	awsdeserializer "github.com/cloudskiff/driftctl/pkg/resource/aws/deserializer"
	"github.com/cloudskiff/driftctl/pkg/terraform"

	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/lambda/lambdaiface"
	"github.com/sirupsen/logrus"
	"github.com/zclconf/go-cty/cty"
)

type LambdaFunctionSupplier struct {
	reader       terraform.ResourceReader
	deserializer deserializer.CTYDeserializer
	client       lambdaiface.LambdaAPI
	runner       *terraform.ParallelResourceReader
	alerter      *alerter.Alerter
}

func NewLambdaFunctionSupplier(runner *parallel.ParallelRunner, client lambdaiface.LambdaAPI, alerter *alerter.Alerter) *LambdaFunctionSupplier {
	return &LambdaFunctionSupplier{
		terraform.Provider(terraform.AWS),
		awsdeserializer.NewLambdaFunctionDeserializer(),
		client,
		terraform.NewParallelResourceReader(runner),
		alerter,
	}
}

func (s LambdaFunctionSupplier) Resources() ([]resource.Resource, error) {
	functions, err := listLambdaFunctions(s.client)
	if err != nil {
		handled := handleListAwsError(err, resourceaws.AwsLambdaFunctionResourceType, s.alerter)
		if !handled {
			return nil, err
		}
		return []resource.Resource{}, nil
	}
	results := make([]cty.Value, 0)
	if len(functions) > 0 {
		for _, function := range functions {
			fun := *function
			s.runner.Run(func() (cty.Value, error) {
				return s.readLambda(fun)
			})
		}
		results, err = s.runner.Wait()
		if err != nil {
			return nil, err
		}
	}
	return s.deserializer.Deserialize(results)
}

func (s LambdaFunctionSupplier) readLambda(function lambda.FunctionConfiguration) (cty.Value, error) {
	name := *function.FunctionName
	resFunction, err := s.reader.ReadResource(
		terraform.ReadResourceArgs{
			Ty: resourceaws.AwsLambdaFunctionResourceType,
			ID: name,
			Attributes: map[string]string{
				"function_name": name,
			},
		},
	)
	if err != nil {
		logrus.Warnf("Error reading function %s[%s]: %+v", name, resourceaws.AwsLambdaFunctionResourceType, err)
		return cty.NilVal, err
	}

	return *resFunction, nil
}

func listLambdaFunctions(client lambdaiface.LambdaAPI) ([]*lambda.FunctionConfiguration, error) {
	var functions []*lambda.FunctionConfiguration
	input := &lambda.ListFunctionsInput{}
	err := client.ListFunctionsPages(input, func(res *lambda.ListFunctionsOutput, lastPage bool) bool {
		functions = append(functions, res.Functions...)
		return !lastPage
	})
	if err != nil {
		return nil, err
	}
	return functions, nil
}
