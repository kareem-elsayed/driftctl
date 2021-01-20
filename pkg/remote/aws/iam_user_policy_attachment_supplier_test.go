package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"

	"github.com/cloudskiff/driftctl/pkg/alerter"
	"github.com/cloudskiff/driftctl/pkg/parallel"

	awsdeserializer "github.com/cloudskiff/driftctl/pkg/resource/aws/deserializer"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/service/iam"

	"github.com/cloudskiff/driftctl/test/goldenfile"
	mocks2 "github.com/cloudskiff/driftctl/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/cloudskiff/driftctl/mocks"

	"github.com/cloudskiff/driftctl/pkg/resource"
	"github.com/cloudskiff/driftctl/pkg/terraform"
	"github.com/cloudskiff/driftctl/test"
)

func TestIamUserPolicyAttachmentSupplier_Resources(t *testing.T) {

	cases := []struct {
		test      string
		dirName   string
		mocks     func(client *mocks.FakeIAM)
		wantAlert alerter.Alerts
		err       error
	}{
		{
			test:    "iam multiples users multiple policies",
			dirName: "iam_user_policy_attachment_multiple",
			mocks: func(client *mocks.FakeIAM) {
				client.On("ListUsersPages",
					&iam.ListUsersInput{},
					mock.MatchedBy(func(callback func(res *iam.ListUsersOutput, lastPage bool) bool) bool {
						callback(&iam.ListUsersOutput{Users: []*iam.User{
							{
								UserName: aws.String("loadbalancer"),
							},
							{
								UserName: aws.String("loadbalancer2"),
							},
							{
								UserName: aws.String("loadbalancer3"),
							},
						}}, true)
						return true
					})).Return(nil).Once()

				shouldSkipfirst := false
				shouldSkipSecond := false
				shouldSkipThird := false

				client.On("ListAttachedUserPoliciesPages",
					&iam.ListAttachedUserPoliciesInput{
						UserName: aws.String("loadbalancer"),
					},
					mock.MatchedBy(func(callback func(res *iam.ListAttachedUserPoliciesOutput, lastPage bool) bool) bool {
						if shouldSkipfirst {
							return false
						}
						callback(&iam.ListAttachedUserPoliciesOutput{AttachedPolicies: []*iam.AttachedPolicy{
							&iam.AttachedPolicy{
								PolicyArn:  aws.String("arn:aws:iam::526954929923:policy/test"),
								PolicyName: aws.String("test-attach"),
							},
							&iam.AttachedPolicy{
								PolicyArn:  aws.String("arn:aws:iam::526954929923:policy/test2"),
								PolicyName: aws.String("test-attach2"),
							},
							&iam.AttachedPolicy{
								PolicyArn:  aws.String("arn:aws:iam::526954929923:policy/test3"),
								PolicyName: aws.String("test-attach3"),
							},
						}}, false)
						callback(&iam.ListAttachedUserPoliciesOutput{AttachedPolicies: []*iam.AttachedPolicy{
							&iam.AttachedPolicy{
								PolicyArn:  aws.String("arn:aws:iam::526954929923:policy/test4"),
								PolicyName: aws.String("test-attach4"),
							},
						}}, true)
						shouldSkipfirst = true
						return true
					})).Return(nil).Once()

				client.On("ListAttachedUserPoliciesPages",
					&iam.ListAttachedUserPoliciesInput{
						UserName: aws.String("loadbalancer2"),
					},
					mock.MatchedBy(func(callback func(res *iam.ListAttachedUserPoliciesOutput, lastPage bool) bool) bool {
						if shouldSkipSecond {
							return false
						}
						callback(&iam.ListAttachedUserPoliciesOutput{AttachedPolicies: []*iam.AttachedPolicy{
							&iam.AttachedPolicy{
								PolicyArn:  aws.String("arn:aws:iam::526954929923:policy/test"),
								PolicyName: aws.String("test-attach"),
							},
							&iam.AttachedPolicy{
								PolicyArn:  aws.String("arn:aws:iam::526954929923:policy/test2"),
								PolicyName: aws.String("test-attach2"),
							},
							&iam.AttachedPolicy{
								PolicyArn:  aws.String("arn:aws:iam::526954929923:policy/test3"),
								PolicyName: aws.String("test-attach3"),
							},
						}}, false)
						callback(&iam.ListAttachedUserPoliciesOutput{AttachedPolicies: []*iam.AttachedPolicy{
							&iam.AttachedPolicy{
								PolicyArn:  aws.String("arn:aws:iam::526954929923:policy/test4"),
								PolicyName: aws.String("test-attach4"),
							},
						}}, true)
						shouldSkipSecond = true
						return true
					})).Return(nil).Once()

				client.On("ListAttachedUserPoliciesPages",
					&iam.ListAttachedUserPoliciesInput{
						UserName: aws.String("loadbalancer3"),
					},
					mock.MatchedBy(func(callback func(res *iam.ListAttachedUserPoliciesOutput, lastPage bool) bool) bool {
						if shouldSkipThird {
							return false
						}
						callback(&iam.ListAttachedUserPoliciesOutput{AttachedPolicies: []*iam.AttachedPolicy{
							&iam.AttachedPolicy{
								PolicyArn:  aws.String("arn:aws:iam::526954929923:policy/test"),
								PolicyName: aws.String("test-attach"),
							},
							&iam.AttachedPolicy{
								PolicyArn:  aws.String("arn:aws:iam::526954929923:policy/test2"),
								PolicyName: aws.String("test-attach2"),
							},
							&iam.AttachedPolicy{
								PolicyArn:  aws.String("arn:aws:iam::526954929923:policy/test3"),
								PolicyName: aws.String("test-attach3"),
							},
						}}, false)
						shouldSkipThird = true
						return true
					})).Return(nil).Once()

			},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:    "cannot list user policies",
			dirName: "iam_user_policy_empty",
			mocks: func(client *mocks.FakeIAM) {
				client.On("ListUsersPages",
					&iam.ListUsersInput{},
					mock.MatchedBy(func(callback func(res *iam.ListUsersOutput, lastPage bool) bool) bool {
						return true
					})).Return(awserr.NewRequestFailure(nil, 403, "")).Once()
			},
			wantAlert: alerter.Alerts{"aws_iam_user_policy_attachment": []alerter.Alert{alerter.Alert{Message: "Ignoring aws_iam_user_policy_attachment from drift calculation. Listing aws_iam_user is forbidden.", ShouldIgnoreResource: true}}},
			err:       nil,
		},
	}
	for _, c := range cases {
		alertr := alerter.NewAlerter()
		shouldUpdate := c.dirName == *goldenfile.Update
		if shouldUpdate {
			provider, err := NewTerraFormProvider()
			if err != nil {
				t.Fatal(err)
			}

			terraform.AddProvider(terraform.AWS, provider)
			resource.AddSupplier(NewIamUserPolicyAttachmentSupplier(provider.Runner(), iam.New(provider.session), alertr))
		}

		t.Run(c.test, func(tt *testing.T) {
			fakeIam := mocks.FakeIAM{}
			c.mocks(&fakeIam)

			provider := mocks2.NewMockedGoldenTFProvider(c.dirName, terraform.Provider(terraform.AWS), shouldUpdate)
			deserializer := awsdeserializer.NewIamUserPolicyAttachmentDeserializer()
			s := &IamUserPolicyAttachmentSupplier{
				provider,
				deserializer,
				&fakeIam,
				terraform.NewParallelResourceReader(parallel.NewParallelRunner(context.TODO(), 1)),
				alertr,
			}
			got, err := s.Resources()
			if c.err != err {
				t.Errorf("Expected error %+v got %+v", c.err, err)
			}

			mock.AssertExpectationsForObjects(tt)
			assert.Equal(t, c.wantAlert, alertr.Retrieve())
			test.CtyTestDiff(got, c.dirName, provider, awsdeserializer.NewIamPolicyAttachmentDeserializer(), shouldUpdate, t)
		})
	}
}
