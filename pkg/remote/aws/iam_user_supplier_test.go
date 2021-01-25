package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"

	"github.com/cloudskiff/driftctl/pkg/alerter"
	"github.com/cloudskiff/driftctl/pkg/parallel"

	awsdeserializer "github.com/cloudskiff/driftctl/pkg/resource/aws/deserializer"

	"github.com/cloudskiff/driftctl/test/goldenfile"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/service/iam"

	mocks2 "github.com/cloudskiff/driftctl/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/cloudskiff/driftctl/mocks"

	"github.com/cloudskiff/driftctl/pkg/resource"
	"github.com/cloudskiff/driftctl/pkg/terraform"
	"github.com/cloudskiff/driftctl/test"
)

func TestIamUserSupplier_Resources(t *testing.T) {

	cases := []struct {
		test      string
		dirName   string
		mocks     func(client *mocks.FakeIAM)
		wantAlert alerter.Alerts
		err       error
	}{
		{
			test:    "no iam user",
			dirName: "iam_user_empty",
			mocks: func(client *mocks.FakeIAM) {
				client.On("ListUsersPages", mock.Anything, mock.Anything).Return(nil)
			},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:    "iam multiples users",
			dirName: "iam_user_multiple",
			mocks: func(client *mocks.FakeIAM) {
				client.On("ListUsersPages",
					&iam.ListUsersInput{},
					mock.MatchedBy(func(callback func(res *iam.ListUsersOutput, lastPage bool) bool) bool {
						callback(&iam.ListUsersOutput{Users: []*iam.User{
							{
								UserName: aws.String("test-driftctl-0"),
							},
							{
								UserName: aws.String("test-driftctl-1"),
							},
						}}, false)
						callback(&iam.ListUsersOutput{Users: []*iam.User{
							{
								UserName: aws.String("test-driftctl-2"),
							},
						}}, true)
						return true
					})).Return(nil)
			},
			wantAlert: alerter.Alerts{},
			err:       nil,
		},
		{
			test:    "cannot list iam user",
			dirName: "iam_user_empty",
			mocks: func(client *mocks.FakeIAM) {
				client.On("ListUsersPages", mock.Anything, mock.Anything).Return(awserr.NewRequestFailure(nil, 403, ""))
			},
			wantAlert: alerter.Alerts{"aws_iam_user": []alerter.Alert{alerter.Alert{Message: "Ignoring aws_iam_user from drift calculation: Listing aws_iam_user is forbidden.", ShouldIgnoreResource: true}}},
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
			resource.AddSupplier(NewIamUserSupplier(provider.Runner(), iam.New(provider.session), alertr))
		}

		t.Run(c.test, func(tt *testing.T) {
			fakeIam := mocks.FakeIAM{}
			c.mocks(&fakeIam)

			provider := mocks2.NewMockedGoldenTFProvider(c.dirName, terraform.Provider(terraform.AWS), shouldUpdate)
			deserializer := awsdeserializer.NewIamUserDeserializer()
			s := &IamUserSupplier{
				provider,
				deserializer,
				&fakeIam,
				terraform.NewParallelResourceReader(parallel.NewParallelRunner(context.TODO(), 10)),
				alertr,
			}
			got, err := s.Resources()
			if c.err != err {
				t.Errorf("Expected error %+v got %+v", c.err, err)
			}

			mock.AssertExpectationsForObjects(tt)
			assert.Equal(t, c.wantAlert, alertr.Retrieve())
			test.CtyTestDiff(got, c.dirName, provider, deserializer, shouldUpdate, t)
		})
	}
}
