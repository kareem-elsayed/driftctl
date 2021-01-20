package aws

import (
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/cloudskiff/driftctl/pkg/alerter"
	"github.com/cloudskiff/driftctl/pkg/parallel"

	"github.com/cloudskiff/driftctl/pkg/remote/deserializer"
	"github.com/cloudskiff/driftctl/pkg/resource"
	resourceaws "github.com/cloudskiff/driftctl/pkg/resource/aws"
	awsdeserializer "github.com/cloudskiff/driftctl/pkg/resource/aws/deserializer"
	"github.com/cloudskiff/driftctl/pkg/terraform"

	"github.com/sirupsen/logrus"
	"github.com/zclconf/go-cty/cty"
)

type IamUserPolicyAttachmentSupplier struct {
	reader       terraform.ResourceReader
	deserializer deserializer.CTYDeserializer
	client       iamiface.IAMAPI
	runner       *terraform.ParallelResourceReader
	alerter      *alerter.Alerter
}

func NewIamUserPolicyAttachmentSupplier(runner *parallel.ParallelRunner, client iamiface.IAMAPI, alerter *alerter.Alerter) *IamUserPolicyAttachmentSupplier {
	return &IamUserPolicyAttachmentSupplier{
		terraform.Provider(terraform.AWS),
		awsdeserializer.NewIamUserPolicyAttachmentDeserializer(),
		client,
		terraform.NewParallelResourceReader(runner),
		alerter,
	}
}

func (s IamUserPolicyAttachmentSupplier) Resources() ([]resource.Resource, error) {
	users, err := listIamUsers(s.client)
	if err != nil {
		handled := handleListAwsErrorWithMessage(err, resourceaws.AwsIamUserPolicyAttachmentResourceType, s.alerter, resourceaws.AwsIamUserResourceType)
		if !handled {
			return nil, err
		}
		return []resource.Resource{}, nil
	}
	results := make([]cty.Value, 0)
	if len(users) > 0 {
		attachedPolicies := make([]*attachedUserPolicy, 0)
		for _, user := range users {
			userName := *user.UserName
			policyAttachmentList, err := listIamUserPoliciesAttachment(userName, s.client)
			if err != nil {
				handled := handleListAwsError(err, resourceaws.AwsIamUserPolicyAttachmentResourceType, s.alerter)
				if !handled {
					return nil, err
				}
			}
			attachedPolicies = append(attachedPolicies, policyAttachmentList...)
		}

		for _, attachedPolicy := range attachedPolicies {
			attached := *attachedPolicy
			s.runner.Run(func() (cty.Value, error) {
				return s.readRes(attached)
			})
		}
		results, err = s.runner.Wait()
		if err != nil {
			return nil, err
		}
	}

	return s.deserializer.Deserialize(results)
}

func (s IamUserPolicyAttachmentSupplier) readRes(attachedPol attachedUserPolicy) (cty.Value, error) {
	res, err := s.reader.ReadResource(
		terraform.ReadResourceArgs{
			Ty: resourceaws.AwsIamUserPolicyAttachmentResourceType,
			ID: *attachedPol.PolicyName,
			Attributes: map[string]string{
				"user":       attachedPol.Username,
				"policy_arn": *attachedPol.PolicyArn,
			},
		},
	)

	if err != nil {
		logrus.Warnf("Error reading iam user policy attachment %s[%s]: %+v", attachedPol, resourceaws.AwsIamUserPolicyAttachmentResourceType, err)
		return cty.NilVal, err
	}
	return *res, nil
}

func listIamUserPoliciesAttachment(username string, client iamiface.IAMAPI) ([]*attachedUserPolicy, error) {
	var attachedUserPolicies []*attachedUserPolicy
	input := &iam.ListAttachedUserPoliciesInput{
		UserName: &username,
	}
	err := client.ListAttachedUserPoliciesPages(input, func(res *iam.ListAttachedUserPoliciesOutput, lastPage bool) bool {
		for _, policy := range res.AttachedPolicies {
			attachedUserPolicies = append(attachedUserPolicies, &attachedUserPolicy{
				AttachedPolicy: *policy,
				Username:       username,
			})
		}
		return !lastPage
	})
	if err != nil {
		return nil, err
	}
	return attachedUserPolicies, nil
}

type attachedUserPolicy struct {
	iam.AttachedPolicy
	Username string
}
