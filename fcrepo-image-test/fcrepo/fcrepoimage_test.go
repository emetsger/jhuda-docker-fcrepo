package main

import (
	"crypto/tls"
	"fcrepo-image-test/fcrepo/env"
	"fmt"
	"github.com/saopayne/gsoup"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

var fcrepoEnv = env.New()

var deps = map[string]bool{
	"ldap:389":       false,
	"idp:8443":       false,
	"sp:443":         false,
	"activemq:5672":  false,
	"activemq:61613": false,
	"activemq:61616": false,
}

func TestMain(m *testing.M) {
	// wait for Fedora to be up (which in and of itself is a test)
	start := time.Now()
	var elapsed time.Duration
	timeout := time.Second * 60
	rc := -1
	for (rc > 499 || rc < 0) && !timedout(start, timeout) {
		if res, err := http.Get(fcrepoEnv.BaseUri); err == nil {
			rc = res.StatusCode
			defer func() { _ = res.Body.Close() }()
		}
		//fmt.Printf("%v Waiting for %v ...\n", time.Now(), fcrepoEnv.BaseUri)
		time.Sleep(5 * time.Second)
		elapsed = time.Now().Sub(start)
	}

	if rc > 499 || rc < 0 {
		fmt.Printf("Fedora did not start successfully: %v (elapsed: %v)\n", rc, elapsed)
		os.Exit(rc)
	}

	//fmt.Printf("Fedora started successfully: %v (elapsed: %v)\n", rc, elapsed)

	// Verify tcp connectivity to dependencies
	for k := range deps {
		start = time.Now()
		for !timedout(start, timeout) {
			//fmt.Printf("Dialing %v\n", k)
			if c, err := net.Dial("tcp", k); err == nil {
				_ = c.Close()
				deps[k] = true
				//fmt.Printf("Successfully connected to %v\n", k)
				break
			} else {
				time.Sleep(5 * time.Second)
			}
		}
	}

	for k, v := range deps {
		if !v {
			fmt.Printf("failed to connect to %v", k)
			os.Exit(1)
		}
	}

	os.Exit(m.Run())
}

// returns true if the current time minus the start time is greater than the timeout duration
func timedout(start time.Time, timeout time.Duration) bool {
	return time.Now().Sub(start) > timeout
}

// insures the rest API requires authentication by default
func Test_BasicAuthChallenge(t *testing.T) {
	res, err := http.Get(fcrepoEnv.BaseUri)
	assert.Nil(t, err)
	defer func() { _ = res.Body.Close() }()

	assert.Equal(t, 401, res.StatusCode)
}

// insures the environment's username and password authenticates successfully to the Fedora REST API
func Test_UserPassOk(t *testing.T) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", fcrepoEnv.BaseUri, nil)
	req.SetBasicAuth(fcrepoEnv.User, fcrepoEnv.Password)

	res, err := client.Do(req)
	assert.Nil(t, err)
	defer func() { _ = res.Body.Close() }()

	assert.EqualValues(t, 200, res.StatusCode)
}

// accessing the repository via the shibboleth SP without providing authentication should result in redirection to the
// login form
func Test_SpAuthChallenge(t *testing.T) {
	// Dangerous but don't verify the server cert
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	res, err := client.Get(fcrepoEnv.PublicBaseUri)
	assert.Nilf(t, err, "Error loading %v: %v", fcrepoEnv.PublicBaseUri, err)
	defer func() { _ = res.Body.Close() }()

	assert.Equal(t, 200, res.StatusCode)
	assert.Equal(t, "text/html;charset=utf-8", res.Header.Get("Content-Type"))

	b, err := ioutil.ReadAll(res.Body)

	doc := gsoup.HTMLParse(string(b))
	title := doc.Find("title").Text()
	assert.Equal(t, "Web Login Service", title)

	// TODO a test for submitting the form and verifying login
	/*
		/html/body/div/div/div/div[1]/form

		<form action="/idp/profile/SAML2/Redirect/SSO?execution=e1s1" method="post">


		                          <div class="form-element-wrapper">
		                <label for="username">Username</label>
		                <input class="form-element form-field" id="username" name="j_username" type="text" value="">
		              </div>

		              <div class="form-element-wrapper">
		                <label for="password">Password</label>
		                <input class="form-element form-field" id="password" name="j_password" type="password" value="">
		              </div>

		                                          <div class="form-element-wrapper">
		                <input type="checkbox" name="donotcache" value="1" id="donotcache">
		                <label for="donotcache">Don't Remember Login</label>
		               </div>


		              <div class="form-element-wrapper">
		                <input id="_shib_idp_revokeConsent" type="checkbox" name="_shib_idp_revokeConsent" value="true">
		                <label for="_shib_idp_revokeConsent">Clear prior granting of permission for release of your information to this service.</label>
		              </div>

		                          <div class="form-element-wrapper">
		                <button class="form-element form-button" type="submit" name="_eventId_proceed" onclick="this.childNodes[0].nodeValue='Logging in, please wait...'">Login</button>
		              </div>

		                        </form>
		<button class="form-element form-button" type="submit" name="_eventId_proceed" onclick="this.childNodes[0].nodeValue='Logging in, please wait...'">Login</button>
	*/
}