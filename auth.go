package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
)

// VerifyRequestAuth returns true if we are logged in
func VerifyRequestAuth(r *http.Request, secret string) (bool, error) {
	if cookie, err := r.Cookie("jwt"); err == nil {
		token, err := jwt.ParseWithClaims(cookie.Value, &AdminClaims{},
			func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(secret), nil
			})
		if claims, ok := token.Claims.(*AdminClaims); ok && token.Valid {
			if claims.Admin {
				return true, nil
			}
		} else {
			return false, fmt.Errorf("Unknown error parsing validation cookie: %s", err.Error())
		}
	}

	return false, nil
}

// ValidateRequestAuth scans the request for credentials and generates a auth token
func ValidateRequestAuth(w http.ResponseWriter, r *http.Request, secret string, validDigest string) error {
	r.ParseForm()

	creds, ok := r.Form["credentials"]
	if !ok || len(creds) < 1 {
		return errors.New("No credentials provided")
	}

	validDigestBytes, err := hex.DecodeString(validDigest)
	if err != nil {
		return err
	}

	digest := sha256.Sum256([]byte(creds[0]))

	time.Sleep(2*time.Second + time.Duration(rand.Intn(2000)-1000)*time.Millisecond)
	if subtle.ConstantTimeCompare(validDigestBytes, digest[:]) == 1 {

		// Authentication successful; set cookie
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, AdminClaims{Admin: true})
		tokenString, err := token.SignedString([]byte(secret))
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
