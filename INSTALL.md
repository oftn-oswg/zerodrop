# Zerodrop Installation

Zerodrop runs as a single binary (like most Go programs) that references the
`templates` directory in the current working directory.

## Simple

Use the following commands for a quick and easy demo of Zerodrop.

```sh
go get github.com/oftn-oswg/zerodrop
cd $GOPATH/src/oftn-oswg/zerodrop

yarn install # or npm install
yarn run css # or npm run css
yarn run dist # or npm run dist

# EDIT config.yml to your liking.
# REMEMBER to set up your secrets:
# 1) authsecret: cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1
# 2) authdigest: echo -n YOUR_PASSWORD_HERE | sha256sum | cut -c -64

$GOPATH/bin/zerodrop -config config.yml
```

## Systemd

Use the following commands for a more production-ready setup with **systemd** on
Ubuntu. This assumes the following.
- You have created a user called `zerodrop` with the home directory of `/home/zerodrop`.
- You have the source downloaded into `$GOPATH/src/github.com/oftn-oswg/zerodrop/`.
- You have installed the binary into `$GOPATH/bin/zerodrop`.

### As zerodrop user

```sh
# Navigate to /home/zerodrop
cd

ZERODROP_NAME=drop.example.com

# Create a custom configuration directory
mkdir -p ~/$ZERODROP_NAME/uploads/

# Copy configuration and binary
cp $GOPATH/bin/zerodrop ~/$ZERODROP_NAME/zerodrop
cp $GOPATH/src/github.com/oftn-oswg/zerodrop/config.yml ~/$ZERODROP_NAME/config.yml

# EDIT config.yml to your liking.
# 1) Set upload directory to /home/zerodrop/$ZERODROP_NAME/uploads/
# 2) Set db.source to /home/zerodrop/$ZERODROP_NAME/zerodrop.db
# 3) ...and more...
# REMEMBER to set up your secrets:
# 1) authsecret: cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1
# 2) authdigest: echo -n YOUR_PASSWORD_HERE | sha256sum | cut -c -64
vim ~/$ZERODROP_NAME/config.yml
```

### As root

```sh
# Copy systemd unit template
cp $GOPATH/src/github.com/oftn-oswg/zerodrop/systemd/zerodrop@.service \
    /etc/systemd/system

# INSPECT this file for discrepancies
vim /etc/system/system/zerodrop@.service

# Reload daemon
systemctl daemon-reload

# Enable on startup
systemctl enable zerodrop@$ZERODROP_NAME.service

# Start
systemctl start zerodrop@$ZERODROP_NAME.service
```
