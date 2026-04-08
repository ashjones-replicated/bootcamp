## bootcamp project

core of the project will https://codeberg.org/sparrow/upload which is cloned into the upload folder.

we will be creating a new web interface that leverages its APIs to share files, and perform user management.

the users will be stored in a postgres database along with any API tokens from the upload service

the code for the new web interface will live in ./web

we also need to write a helm chart for this application so that we can deploy it in kubernetes, it needs to follow kubernetes best practices and have liveness and readiness probes, and sensible memory limits. but _not_ CPU limits, those are an antipattern.

the helm chart will live in ./helm

## application design

the web application backend should be written in go, and the web frontend should be written as simply as possible, preferably using basic html, css and javascript.

the database for the webapp should allow mapping upload service api tokens to users for the webapp as well as a store password credentials for the user to log in with.

the web application itself should present a simple view for uploading files with the options provided by the upload service,

it should also feature a sidebar on the right of the screen with a list of already shared files
