package firestylesv

import (
	"net/http"
	//	"net/url"

	//	"errors"

	"github.com/firefirestyle/go.miniblob"
	"github.com/firefirestyle/go.minioauth/twitter"
	"github.com/firefirestyle/go.miniprop"
	"github.com/firefirestyle/go.minisession"
	"github.com/firefirestyle/go.miniuser"
	//
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	//
	//"crypto/rand"
	//"encoding/binary"
	//"strconv"
	"errors"
	"io/ioutil"
	"strings"

	//	"google.golang.org/appengine/blobstore"
)

const (
	UrlTwitterTokenUrlRedirect_callbackUrl              = "cb"
	UrlTwitterTokenUrlRedirect_errorNotFoundCallbackUrl = "1001"
	UrlTwitterTokenUrlRedirect_errorFailedToMakeToken   = "1002"
	UrlTwitterTokenCallback_callbackUrl                 = "cb"
)

const (
	UrlTwitterTokenUrlRedirect = "/api/v1/twitter/tokenurl/redirect"
	UrlTwitterTokenCallback    = "/api/v1/twitter/tokenurl/callback"

	UrlBlobRequestUrl = "/api/v1/blob/requesturl"
	UrlBlobCallback   = "/api/v1/blob/callback"

	UrlUserGet  = "/api/v1/user/get"
	UrlMeLogout = "/api/v1/me/logout"

//	UrlMeUpdateIcon = "/api/v1/me/update-icon"
)

var twitterHandlerObj *twitter.TwitterHandler = nil
var blobHandlerObj *miniblob.BlobHandler = nil
var sessionMgrObj *minisession.SessionManager = nil
var userHandlerObj *miniuser.UserHandler = nil

func GetUserMgrObj(ctx context.Context) *miniuser.UserHandler {
	if userHandlerObj == nil {
		userHandlerObj = miniuser.NewUserHandler(miniuser.UserManagerConfig{
			ProjectId:   "firefirestyle",
			UserKind:    "user",
			RelayIdKind: "relayId",
		}, miniuser.UserHandlerOnEvent{})
	}
	return userHandlerObj
}

func GetSessionMgrObj(ctx context.Context) *minisession.SessionManager {
	if sessionMgrObj == nil {
		sessionMgrObj = minisession.NewSessionManager(minisession.SessionManagerConfig{
			ProjectId: "firefirestyle",
			Kind:      "session",
		})
	}
	return sessionMgrObj
}

func GetBlobHandlerObj(ctx context.Context) *miniblob.BlobHandler {
	if blobHandlerObj == nil {
		blobHandlerObj = miniblob.NewBlobHandler(
			UrlBlobCallback, appengine.VersionID(ctx), //
			miniblob.BlobManagerConfig{
				ProjectId:   "firefirestyle",
				Kind:        "blobstore",
				CallbackUrl: UrlBlobCallback,
			},
			miniblob.BlobHandlerOnEvent{
				OnRequest: func(w http.ResponseWriter, r *http.Request, outputProp *miniprop.MiniProp, blobHandlerObj *miniblob.BlobHandler) (string, map[string]string, error) {
					//
					// login check
					bodyBytes, _ := ioutil.ReadAll(r.Body)
					propObj := miniprop.NewMiniPropFromJson(bodyBytes)
					token := propObj.GetString("token", "")
					ctx := appengine.NewContext(r)

					loginCheckInfo := GetSessionMgrObj(ctx).CheckLoginId(ctx, token, minisession.MakeAccessTokenConfigFromRequest(r))
					if loginCheckInfo.IsLogin == false {
						return "", nil, errors.New("failed to wrong token : (1)")
					}
					//
					// path check
					dir := r.URL.Query().Get("dir")
					if true == strings.HasPrefix(dir, "/user") {
						if false == strings.HasPrefix(dir, "/user/"+loginCheckInfo.AccessTokenObj.GetUserName()) {
							return "", nil, errors.New("failed to wrong token : (2)")
						}
					} else {
						return "", nil, errors.New("failed to wrong token : (3)")
					}
					return loginCheckInfo.AccessTokenObj.GetLoginId(), map[string]string{}, nil
				},
				OnComplete: func(w http.ResponseWriter, r *http.Request, outputProp *miniprop.MiniProp, blobHandlerObj *miniblob.BlobHandler, blobObj *miniblob.BlobItem) error {
					dir := r.URL.Query().Get("dir")
					if true == strings.HasPrefix(dir, "/user") {
						ctx := appengine.NewContext(r)
						userName := strings.Replace(dir, "/user/", "", -1)
						userMgrObj := GetUserMgrObj(ctx)
						userObj, userErr := userMgrObj.GetManager().GetUserFromUserName(ctx, userName)
						if userErr != nil {
							outputProp.SetString("error", "not found user")
							return userErr
						}
						userObj.SetIconUrl("key://" + blobObj.GetBlobKey())
						return nil
					} else {
						return errors.New("unsupport")
					}
				},
			})
	}
	return blobHandlerObj
}

func GetTwitterHandlerObj(ctx context.Context) *twitter.TwitterHandler {
	if twitterHandlerObj == nil {
		twitterHandlerObj = twitter.NewTwitterHandler( //
			"http://"+appengine.DefaultVersionHostname(ctx)+""+UrlTwitterTokenCallback, twitter.TwitterOAuthConfig{
				ConsumerKey:       TwitterConsumerKey,
				ConsumerSecret:    TwitterConsumerSecret,
				AccessToken:       TwitterAccessToken,
				AccessTokenSecret: TwitterAccessTokenSecret}, twitter.TwitterHundlerOnEvent{
				OnFoundUser: func(w http.ResponseWriter, r *http.Request, handler *twitter.TwitterHandler, accesssToken *twitter.SendAccessTokenResult) map[string]string {
					ctx := appengine.NewContext(r)
					sessionMgrObj := GetSessionMgrObj(ctx)

					userMgrObj := GetUserMgrObj(ctx)
					//_, userSessionObj, userObj. :=
					_, _, userObj, err1 := userMgrObj.GetManager().LoginRegistFromTwitter(ctx, //
						accesssToken.GetScreenName(), //
						accesssToken.GetUserID(),     //
						accesssToken.GetOAuthToken()) //
					if err1 != nil {
						return map[string]string{"errcode": "2", "errindo": err1.Error()}
					}
					tokenObj, err := sessionMgrObj.Login(ctx, //
						userObj.GetUserName(), //
						minisession.MakeAccessTokenConfigFromRequest(r))
					if err != nil {
						return map[string]string{"errcode": "1"}
					} else {
						return map[string]string{"token": "" + tokenObj.GetLoginId(), "userName": userObj.GetUserName()}
					}
				},
			})
	}
	return twitterHandlerObj
}

func init() {
	initApi()
	initHomepage()
}

func initHomepage() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Welcome to FireFireStyle!!"))
	})
}

func initApi() {
	http.HandleFunc(UrlTwitterTokenUrlRedirect, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		GetTwitterHandlerObj(appengine.NewContext(r)).TwitterLoginEntry(w, r)
	})
	http.HandleFunc(UrlTwitterTokenCallback, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		GetTwitterHandlerObj(appengine.NewContext(r)).TwitterLoginExit(w, r)
	})
	http.HandleFunc(UrlBlobRequestUrl, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		GetBlobHandlerObj(appengine.NewContext(r)).BlobRequestToken(w, r)
	})

	http.HandleFunc(UrlBlobCallback, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		GetBlobHandlerObj(appengine.NewContext(r)).HandleUploaded(w, r)
	})

	http.HandleFunc(UrlUserGet, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		GetUserMgrObj(appengine.NewContext(r)).HandleGet(w, r)
	})
	http.HandleFunc(UrlMeLogout, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		bodyBytes, _ := ioutil.ReadAll(r.Body)
		propObj := miniprop.NewMiniPropFromJson(bodyBytes)
		token := propObj.GetString("token", "")
		ctx := appengine.NewContext(r)
		GetSessionMgrObj(ctx).Logout(ctx, token, minisession.MakeAccessTokenConfigFromRequest(r))
	})

}

func Debug(ctx context.Context, message string) {
	log.Infof(ctx, message)
}
