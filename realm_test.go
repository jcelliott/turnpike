package turnpike

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNoAuthentication(t *testing.T) {
	Convey("Given a Realm with no authentication", t, func() {
		realm := Realm{}
		Convey("When a client is authenticated", func() {
			msg, err := realm.authenticate(map[string]interface{}{})
			Convey("Authenticate should return a Welcome message", func() {
				So(err, ShouldEqual, nil)
				So(msg.MessageType(), ShouldEqual, WELCOME)
			})
		})
	})
}

type testBasicAuthenticator struct{}

func (t *testBasicAuthenticator) Authenticate(details map[string]interface{}) (map[string]interface{}, error) {
	if details["password"].(string) == "super-secret" {
		return map[string]interface{}{"check": "testing"}, nil
	}
	return nil, fmt.Errorf("wrong password")
}

func TestBasicAuthenticator(t *testing.T) {
	Convey("Given a Realm with a basic authenticator", t, func() {
		realm := Realm{
			Authenticators: map[string]Authenticator{
				"test": &testBasicAuthenticator{},
			},
		}
		Convey("When a client doesn't provide an authmethod", func() {
			details := map[string]interface{}{
				"password": "password",
			}
			_, err := realm.authenticate(details)
			Convey("Authenticate should return an error", func() {
				So(err, ShouldNotEqual, nil)
			})
		})
		Convey("When a client authenticates with invalid credentials", func() {
			details := map[string]interface{}{
				"password":    "password",
				"authmethods": []interface{}{"test"},
			}
			_, err := realm.authenticate(details)
			Convey("Authenticate should return an error", func() {
				So(err, ShouldNotEqual, nil)
			})
		})
		Convey("When a client authenticates with valid credentials ", func() {
			details := map[string]interface{}{
				"password":    "super-secret",
				"authmethods": []interface{}{"test"},
			}
			msg, err := realm.authenticate(details)
			Convey("Authenticate should return a Welcome message", func() {
				So(err, ShouldEqual, nil)
				So(msg.MessageType(), ShouldEqual, WELCOME)
			})
			Convey("Authenticate should return correct details", func() {
				So(msg.(*Welcome).Details["check"], ShouldEqual, "testing")
			})
		})
	})
}

type testCRAuthenticator struct{}

func (t *testCRAuthenticator) Challenge(details map[string]interface{}) (map[string]interface{}, error) {
	if username, ok := details["username"].(string); !ok {
		return nil, fmt.Errorf("no username given")
	} else {
		return map[string]interface{}{"challenge": username}, nil
	}
}

func (t *testCRAuthenticator) Authenticate(challenge map[string]interface{}, signature string) (map[string]interface{}, error) {
	if signature == testCRSign(challenge) {
		return map[string]interface{}{"check": "testing"}, nil
	}
	return nil, fmt.Errorf("invalid signature, authentication failed")
}

func testCRSign(challenge map[string]interface{}) string {
	return challenge["challenge"].(string) + "123xyz"
}

func TestCRAuthenticator(t *testing.T) {
	Convey("Given a realm with a challenge-response authenticator", t, func() {
		realm := Realm{
			CRAuthenticators: map[string]CRAuthenticator{
				"test": &testCRAuthenticator{},
			},
		}
		Convey("When a client provides invalid details for the authentication type", func() {
			details := map[string]interface{}{
				"authmethods": []interface{}{"test"},
			}
			_, err := realm.authenticate(details)
			Convey("Authenticate should return an error", func() {
				So(err, ShouldNotEqual, nil)
			})
		})
		Convey("When a client provides valid details for the authentication type", func() {
			details := map[string]interface{}{
				"username":    "tester1",
				"authmethods": []interface{}{"test"},
			}
			msg, err := realm.authenticate(details)
			Convey("Authenticate should return a Challenge message", func() {
				So(err, ShouldEqual, nil)
				So(msg.MessageType(), ShouldEqual, CHALLENGE)
			})
			challenge := msg.(*Challenge)
			Convey("When a client provides an invalid signature for the challenge", func() {
				_, err := realm.checkResponse(challenge, &Authenticate{Signature: "bogus"})
				Convey("CheckResponse should return an error", func() {
					So(err, ShouldNotEqual, nil)
				})
			})
			Convey("When a client provides a valid signature for the challenge", func() {
				auth := &Authenticate{Signature: testCRSign(challenge.Extra)}
				msg, err := realm.checkResponse(challenge, auth)
				Convey("CheckResponse should return a Welcome message", func() {
					So(err, ShouldEqual, nil)
					So(msg.MessageType(), ShouldEqual, WELCOME)
				})
			})
		})
	})
}
