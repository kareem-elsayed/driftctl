package aws

import (
	"fmt"

	"github.com/cloudskiff/driftctl/pkg/alerter"
	"github.com/cloudskiff/driftctl/pkg/parallel"

	awsdeserializer "github.com/cloudskiff/driftctl/pkg/resource/aws/deserializer"

	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/cloudskiff/driftctl/pkg/remote/deserializer"
	"github.com/cloudskiff/driftctl/pkg/resource"
	resourceaws "github.com/cloudskiff/driftctl/pkg/resource/aws"
	"github.com/cloudskiff/driftctl/pkg/terraform"

	"github.com/sirupsen/logrus"
	"github.com/zclconf/go-cty/cty"
)

type IamUserPolicySupplier struct {
	reader       terraform.ResourceReader
	deserializer deserializer.CTYDeserializer
	client       iamiface.IAMAPI
	runner       *terraform.ParallelResourceReader
	alerter      *alerter.Alerter
}

func NewIamUserPolicySupplier(runner *parallel.ParallelRunner, client iamiface.IAMAPI, alerter *alerter.Alerter) *IamUserPolicySupplier {
	return &IamUserPolicySupplier{
		terraform.Provider(terraform.AWS),
		awsdeserializer.NewIamUserPolicyDeserializer(),
		client,
		terraform.NewParallelResourceReader(runner),
		alerter,
	}
}

func (s IamUserPolicySupplier) Resources() ([]resource.Resource, error) {
	users, err := listIamUsers(s.client)
	if err != nil {
		handled := handleListAwsErrorWithMessage(err, resourceaws.AwsIamUserPolicyResourceType, s.alerter, resourceaws.AwsIamUserResourceType)
		if !handled {
			return nil, err
		}
		return []resource.Resource{}, nil
	}
	results := make([]cty.Value, 0)
	if len(users) > 0 {
		policies := make([]string, 0)
		for _, user := range users {
			userName := *user.UserName
			policyList, err := listIamUserPolicies(userName, s.client)
			if err != nil {
				handled := handleListAwsError(err, resourceaws.AwsIamUserPolicyResourceType, s.alerter)
				if !handled {
					return nil, err
				}
				return []resource.Resource{}, nil
			}
			for _, polName := range policyList {
				policies = append(policies, fmt.Sprintf("%s:%s", userName, *polName))
			}
		}

		for _, policy := range policies {
			polName := policy
			s.runner.Run(func() (cty.Value, error) {
				return s.readRes(polName)
			})
		}
		results, err = s.runner.Wait()
		if err != nil {
			return nil, err
		}
	}
	return s.deserializer.Deserialize(results)
}

func (s IamUserPolicySupplier) readRes(policyName string) (cty.Value, error) {
	res, err := s.reader.ReadResource(
		terraform.ReadResourceArgs{
			Ty: resourceaws.AwsIamUserPolicyResourceType,
			ID: policyName,
		},
	)
	if err != nil {
		logrus.Warnf("Error reading iam user policy %s[%s]: %+v", policyName, resourceaws.AwsIamUserResourceType, err)
		return cty.NilVal, err
	}

	return *res, nil
}

func listIamUserPolicies(username string, client iamiface.IAMAPI) ([]*string, error) {
	var policyNames []*string
	input := &iam.ListUserPoliciesInput{
		UserName: &username,
	}
	err := client.ListUserPoliciesPages(input, func(res *iam.ListUserPoliciesOutput, lastPage bool) bool {
		policyNames = append(policyNames, res.PolicyNames...)
		return !lastPage
	})
	if err != nil {
		return nil, err
	}
	return policyNames, nil
}
