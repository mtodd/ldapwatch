## Development & Testing Environment

We use the following Docker container to run our testing OpenLDAP service:
https://store.docker.com/community/images/rroemhild/test-openldap

```
docker pull rroemhild/test-openldap
docker run --privileged -d -p 389:389 rroemhild/test-openldap
```
