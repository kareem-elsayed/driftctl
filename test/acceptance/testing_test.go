package acceptance

import (
	"os"
	"reflect"
	"testing"
)

func TestAccTestCase_resolveTerraformEnv(t *testing.T) {

	os.Clearenv()
	os.Setenv("ACC_TEST_VAR", "foobar")
	os.Setenv("TEST_VAR", "barfoo")
	os.Setenv("TEST_VAR_2", "barfoo")

	testCase := AccTestCase{}
	env := testCase.resolveTerraformEnv()
	expected := []string{"TEST_VAR=foobar", "TEST_VAR_2=barfoo"}

	if !reflect.DeepEqual(expected, env) {
		t.Fatalf("Variable env override not working, got: %+v, expected %+v", env, expected)
	}

}
