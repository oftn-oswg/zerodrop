package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
)

type AuthCredentials struct {
	Digest string
	Secret []byte
}

type AuthClaims struct {
	Auth bool `json:"admin"`
	jwt.StandardClaims
}

type AuthHandler struct {
	Success         http.Handler
	Failure         http.Handler
	Credentials     AuthCredentials
	SuccessRedirect string
	FailureRedirect string
}

func (a *AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// If we are attempting to log in, validate attempt.
	if r.Method == "POST" && r.URL.Path == "/login" {
		redirect := "/"
		err := a.validate(w, r)

		if err == nil {
			log.Println("Successful authentication by " + RealRemoteAddr(r))
			if a.SuccessRedirect != "" {
				redirect = a.SuccessRedirect
			}
		} else {
			log.Println("Failed authentication by " + RealRemoteAddr(r))
			if a.FailureRedirect != "" {
				redirect = a.FailureRedirect
			}
		}

		http.Redirect(w, r, redirect, 302)
		return
	}

	// If we are attempting to log out, remove cookie.
	if r.URL.Path == "/logout" {

		http.SetCookie(w, &http.Cookie{
			Name:    "jwt",
			Value:   "",
			Expires: time.Unix(0, 0),
		})

		redirect := "/"
		if a.FailureRedirect != "" {
			redirect = a.FailureRedirect
		}
		http.Redirect(w, r, redirect, 302)
		return
	}

	// Otherwise, affirm that the user is logged in to serve the correct page.
	ok, err := a.verify(r)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if ok {
		a.Success.ServeHTTP(w, r)
		return
	}

	a.Failure.ServeHTTP(w, r)
}

// verify returns true if we are logged in
func (a *AuthHandler) verify(r *http.Request) (bool, error) {
	if cookie, err := r.Cookie("jwt"); err == nil {
		token, err := jwt.ParseWithClaims(cookie.Value, &AuthClaims{},
			func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
				}
				return a.Credentials.Secret, nil
			})
		if claims, ok := token.Claims.(*AuthClaims); ok && token.Valid {
			if claims.Auth {
				return true, nil
			}
		} else {
			return false, fmt.Errorf("Unknown error parsing validation cookie: %s", err.Error())
		}
	}

	return false, nil
}

// ValidateRequestAuth scans the request for credentials and generates a auth token
func (a *AuthHandler) validate(w http.ResponseWriter, r *http.Request) error {
	r.ParseForm()

	creds, ok := r.Form["credentials"]
	if !ok || len(creds) < 1 {
		return errors.New("No credentials provided")
	}

	validDigestBytes, err := hex.DecodeString(a.Credentials.Digest)
	if err != nil {
		return err
	}

	digest := sha256.Sum256([]byte(creds[0]))

	time.Sleep(2*time.Second + time.Duration(rand.Intn(2000)-1000)*time.Millisecond)
	if subtle.ConstantTimeCompare(validDigestBytes, digest[:]) == 1 {

		// Authentication successful; set cookie
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, AuthClaims{Auth: true})
		tokenString, err := token.SignedString(a.Credentials.Secret)
		if err != nil {
			return err
		}

		http.SetCookie(w, &http.Cookie{
			Name:   "jwt",
			Value:  tokenString,
			MaxAge: 24 * 60 * 60, // 1 day
			// Secure: true,
		})

		return nil
	}

	return errors.New("Invalid password")
}
