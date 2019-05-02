![Provision](https://raw.githubusercontent.com/txn2/provision/master/mast.jpg)
[![Provision Release](https://img.shields.io/github/release/txn2/provision.svg)](https://github.com/txn2/provision/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/txn2/provision)](https://goreportcard.com/report/github.com/txn2/provision)
[![GoDoc](https://godoc.org/github.com/txn2/query?status.svg)](https://godoc.org/github.com/txn2/provision)
[![Docker Container Image Size](https://shields.beevelop.com/docker/image/image-size/txn2/provision/latest.svg)](https://hub.docker.com/r/txn2/provision/)
[![Docker Container Layers](https://shields.beevelop.com/docker/image/layers/txn2/provision/latest.svg)](https://hub.docker.com/r/txn2/provision/)

**Provision** is a user and account micro-platform, a highly opinionated building block for TXN2 components. **Provision** defines basic object models that represent the foundation for an account, user and asset. **Provision** is intended as a fundamental dependency of current and future TXN2 platform services.

- Elasticsearch is used as a database for **account**, **user** and **asset** objects.
- Intended for basic storage, retrieval and searching.

![Provision Objects](./objects.png)

**Provision** is intended as in internal service to be accessed by other services. Use a secure
reverse proxy for direct access by system operators.

## Configuration

Configuration is inherited from [txn2/micro](https://github.com/txn2/micro#configuration). The
following configuration is specific to **provision**:

| Flag          | Environment Variable | Description                                                |
|:--------------|:---------------------|:-----------------------------------------------------------|
| -esServer     | ELASTIC_SERVER       | Elasticsearch Server (default "http://elasticsearch:9200") |
| -systemPrefix | SYSTEM_PREFIX        | Prefix for system indices. (default "system_")             |

## Routes

| Method | Route Pattern                       | Description                                                      |
|:-------|:------------------------------------|:-----------------------------------------------------------------|
| GET    | [/prefix](#get-prefix)              | Get the prefix used for Elasticsearch indexes.                   |
| POST   | [/account](#upsert-account)         | Upsert an Account object.                                        |
| GET    | [/account/:id](#get-account)        | Get an Account ojbect by id.                                     |
| POST   | /keyCheck/:id                       | Check if an AccessKey is associated with an account.             |
| POST   | [/searchAccounts](#search-accounts) | Search for Accounts with a Lucene query.                         |
| POST   | [/user](#upsert-user)               | Upsert a User object.                                            |
| GET    | [/user/:id](#get-user)              | Get a User object by id.                                         |
| POST   | [/searchUsers](#search-users)       | Search for Users with a Lucene query.                            |
| POST   | /userHasAccess                      | Post an AccessCheck object with Token to determine basic access. |
| POST   | /userHasAdminAccess                 | Post an AccessCheck object with Token to determine admin access. |
| POST   | /authUser                           | Post Credentials and if valid receive a Token.                   |
| POST   | /asset                              | Upsert an Asset.                                                 |
| GET    | /asset/:id                          | Get an asset by id.                                              |
| POST   | /searchAssets                       | Search for Assets with a Lucene query.                           |


## Development

Testing using Elasticsearch and Kibana in docker compose:
```bash
docker-compose up
```

Run for source:
```bash
go run ./cmd/provisison.go --esServer="http://localhost:9200"
```

## Examples

### Util

#### Get Prefix
```bash
curl http://localhost:8080/prefix
```

### Account

#### Upsert Account
```bash
curl -X POST \
  http://localhost:8080/account \
  -d '{
	"id": "xorg",
	"description": "Organization X is an IOT data collection agency.",
	"display_name": "Organization X",
	"active": true,
    "modules": [
        "telematics",
        "wx",
        "data_science",
        "gpu"
    ]
}'
```

#### Get Account
```bash
curl http://localhost:8080/account/xorg
```

#### Search Accounts
```bash
curl -X POST \
  http://localhost:8080/searchAccounts \
  -d '{
  "query": {
    "match_all": {}
  }
}'
```

### User

#### Upsert User
```bash
curl -X POST \
  http://localhost:8080/user \
  -d '{
	"id": "sysop",
	"description": "Global system operator",
	"display_name": "System Operator",
	"active": true,
	"sysop": true,
	"password": "examplepassword",
	"sections_all": false,
	"sections": [],
	"accounts": [],
	"admin_accounts": []
}'
```

#### Get User
```bash
curl http://localhost:8080/user/sysop
```

#### Search Users
```bash
curl -X POST \
  http://localhost:8080/searchUsers \
  -d '{
  "query": {
    "match_all": {}
  }
}'
```

#### Authenticate User
```bash
curl -X POST \
  http://localhost:8080/authUser \
  -d '{
	"id": "sysop",
	"password": "examplepassword"
}'
```

#### Access Check
```bash
# first get a token
TOKEN=$(curl -s -X POST \
          http://localhost:8080/authUser?raw=true \
          -d '{
        	"id": "sysop",
        	"password": "examplepassword"
        }') && echo $TOKEN
        
# check for basic access
curl -X POST \
  http://localhost:8080/userHasAccess \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
	"sections": ["a","b"],
	"accounts": ["example","example2"]
}'

# check for admin access
curl -X POST \
  http://localhost:8080/userHasAdminAccess \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
	"sections": ["a","b"],
	"accounts": ["example","example2"]
}'
```

## Release Packaging

Build test release:
```bash
goreleaser --skip-publish --rm-dist --skip-validate
```

Build and release:
```bash
GITHUB_TOKEN=$GITHUB_TOKEN goreleaser --rm-dist
```
