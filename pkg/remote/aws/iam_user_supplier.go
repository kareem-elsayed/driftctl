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

type IamUserSupplier struct {
	reader       terraform.ResourceReader
	deserializer deserializer.CTYDeserializer
	client       iamiface.IAMAPI
	runner       *terraform.ParallelResourceReader
	alerter      *alerter.Alerter
}

func NewIamUserSupplier(runner *parallel.ParallelRunner, client iamiface.IAMAPI, alerter *alerter.Alerter) *IamUserSupplier {
	return &IamUserSupplier{
		terraform.Provider(terraform.AWS),
		awsdeserializer.NewIamUserDeserializer(),
		client,
		terraform.NewParallelResourceReader(runner),
		alerter,
	}
}

func (s IamUserSupplier) Resources() ([]resource.Resource, error) {
	users, err := listIamUsers(s.client)
	if err != nil {
		handled := handleListAwsError(err, resourceaws.AwsIamUserResourceType, s.alerter)
		if !handled {
			return nil, err
		}
		return []resource.Resource{}, nil
	}
	results := make([]cty.Value, 0)
	if len(users) > 0 {
		for _, user := range users {
			u := *user
			s.runner.Run(func() (cty.Value, error) {
				return s.readRes(&u)
			})
		}
		results, err = s.runner.Wait()
		if err != nil {
			return nil, err
		}
	}
	return s.deserializer.Deserialize(results)
}

func (s IamUserSupplier) readRes(user *iam.User) (cty.Value, error) {
	res, err := s.reader.ReadResource(
		terraform.ReadResourceArgs{
			Ty: resourceaws.AwsIamUserResourceType,
			ID: *user.UserName,
		},
	)
	if err != nil {
		logrus.Warnf("Error reading iam user %s[%s]: %+v", *user.UserName, resourceaws.AwsIamUserResourceType, err)
		return cty.NilVal, err
	}

	return *res, nil
}

func listIamUsers(client iamiface.IAMAPI) ([]*iam.User, error) {
	var resources []*iam.User
	input := &iam.ListUsersInput{}
	err := client.ListUsersPages(input, func(res *iam.ListUsersOutput, lastPage bool) bool {
		resources = append(resources, res.Users...)
		return !lastPage
	})
	if err != nil {
		return nil, err
	}
	return resources, nil
}
