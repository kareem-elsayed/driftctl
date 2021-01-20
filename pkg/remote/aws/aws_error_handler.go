package aws

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/cloudskiff/driftctl/pkg/alerter"
)

func handleListAwsError(err error, typ string, alertr *alerter.Alerter) (handled bool) {
	return handleListAwsErrorWithMessage(err, typ, alertr, "")
}

func handleListAwsErrorWithMessage(err error, typ string, alertr *alerter.Alerter, forbiddenTyp string) (handled bool) {
	if reqerr, ok := err.(awserr.RequestFailure); ok {
		if reqerr.StatusCode() == 403 {

			message := fmt.Sprintf("Ignoring %s from drift calculation: Listing %s is forbidden.", typ, typ)
			if forbiddenTyp != "" {
				message = fmt.Sprintf("Ignoring %s from drift calculation. Listing %s is forbidden.", typ, forbiddenTyp)
			}
			logrus.Debugf(message)
			alertr.SendAlert(typ, alerter.Alert{
				Message:              message,
				ShouldIgnoreResource: true,
			})
			return true
		}
	}
	return false
}
