package main

import (
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/kataras/iris"
)

// Serve is an authentication middlware function used to authenticate update requests
func (a authMiddleware) Serve(ctx *iris.Context) {
	usernameStr := ctx.RequestHeader("X-Api-User")
	password := ctx.RequestHeader("X-Api-Key")
	postData := ACMETxt{}

	username, err := getValidUsername(usernameStr)
	if err == nil && validKey(password) {
		au, err := DB.GetByUsername(username)
		if err != nil {
			log.WithFields(log.Fields{"error": err.Error()}).Error("Error while trying to get user")
			// To protect against timed side channel (never gonna give you up)
			correctPassword(password, "$2a$10$8JEFVNYYhLoBysjAxe2yBuXrkDojBQBkVpXEQgyQyjn43SvJ4vL36")
		} else {
			if correctPassword(password, au.Password) {
				// Password ok
				if err := ctx.ReadJSON(&postData); err == nil {
					// Check that the subdomain belongs to the user
					if au.Subdomain == postData.Subdomain {
						ctx.Next()
						return
					}
				} else {
					// JSON error
					ctx.JSON(iris.StatusBadRequest, iris.Map{"error": "bad data"})
					return
				}
			} else {
				// Wrong password
				log.WithFields(log.Fields{"username": username}).Warning("Failed password check")
			}
		}
	}
	ctx.JSON(iris.StatusUnauthorized, iris.Map{"error": "unauthorized"})
}

func webRegisterPost(ctx *iris.Context) {
	// Create new user
	nu, err := DB.Register(cidrslice{})
	var regJSON iris.Map
	var regStatus int
	if err != nil {
		errstr := fmt.Sprintf("%v", err)
		regJSON = iris.Map{"error": errstr}
		regStatus = iris.StatusInternalServerError
		log.WithFields(log.Fields{"error": err.Error()}).Debug("Error in registration")
	} else {
		regJSON = iris.Map{"username": nu.Username, "password": nu.Password, "fulldomain": nu.Subdomain + "." + DNSConf.General.Domain, "subdomain": nu.Subdomain}
		regStatus = iris.StatusCreated

		log.WithFields(log.Fields{"user": nu.Username.String()}).Debug("Created new user")
	}
	ctx.JSON(regStatus, regJSON)
}

func webRegisterGet(ctx *iris.Context) {
	// This is placeholder for now
	webRegisterPost(ctx)
}

func webUpdatePost(ctx *iris.Context) {
	// User auth done in middleware
	a := ACMETxt{}
	userStr := ctx.RequestHeader("X-API-User")
	// Already checked in auth middlware
	username, _ := getValidUsername(userStr)
	// Already checked in auth middleware
	_ = ctx.ReadJSON(&a)
	a.Username = username
	// Do update
	if validSubdomain(a.Subdomain) && validTXT(a.Value) {
		err := DB.Update(a)
		if err != nil {
			log.WithFields(log.Fields{"error": err.Error()}).Debug("Error while trying to update record")
			webUpdatePostError(ctx, errors.New("internal error"), iris.StatusInternalServerError)
			return
		}
		ctx.JSON(iris.StatusOK, iris.Map{"txt": a.Value})
	} else {
		log.WithFields(log.Fields{"subdomain": a.Subdomain, "txt": a.Value}).Debug("Bad data for subdomain")
		webUpdatePostError(ctx, errors.New("bad data"), iris.StatusBadRequest)
		return
	}
}

func webUpdatePostError(ctx *iris.Context, err error, status int) {
	errStr := fmt.Sprintf("%v", err)
	updJSON := iris.Map{"error": errStr}
	ctx.JSON(status, updJSON)
}
