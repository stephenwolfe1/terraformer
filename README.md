# terraformer
This is a simple go app that wraps Terraform commands designed to be run in Docker

# Build
`[IMAGE_TAG=local] make build`
This command will build the docker container for local testing

# Docker Testing
This command will run the specified command for the main service example by default.

```bash
COMMAND=[init/validate/plan/apply/destroy] make test
```
 - Optional env varialbes:
```
  IMAGE_TAG               Override the docker image tag
  TYPE                    Run terraform for modules or services
  NAME                    Name of the module or service to run
```

Examples:
```bash
COMMAND=init make test

IMAGE_TAG=test COMMAND=validate TYPE=modules NAME=random make test
```
