
<a name="v0.0.3"></a>
## [v0.0.3](https://github.com/lealre/fs-mcp/compare/v0.0.2...v0.0.3) (2025-12-13)

* Add basic authentication and authorization ([#3](https://github.com/lealre/fs-mcp/issues/3))
* Add missing return statment
* Remove cmd folder from git track
* Add endpoint to get a group
* fix: Json fields to add a comment
* test: Extend group tests to check the user update when creating/adding a user
* Update groups enpoints to when creating a group or adding a user, also updating the user groups records in user collections. Just group owners can add new users to a group
* Update login response to send user info with token and json fields in api ErrorResponse to be camelCase
* test: Add basic tests to delete comments
* Update endpoint to delete comments
* test: Add basic tests to update comments
* Update endpoint to update comments
* fix: Group endpoint name to get comments form a title
* test: Add basic tests to get comments
* Update endpoint to get comments
* test: Add basic tests to add comments
* Update endpoint to add comments
* Update packages versions and rebase
* Refactor note ranghe validation for rating endpoint
* test: Add test to note validation when adding a note
* test: Extend tests to update ratings
* Remove endpoit to get batch of ratings
* test: Start tests for updating a rating
* Update enpoint to update a rating
* test: Add tests for adding ratings
* Update method to add rating to check group/title combination
* test: Refactor test setup in new reusable methods
* test: Add tests to setting a movie as watched related to auth
* fix: Groups endpoints related to permissions
* test: Extend groups endpoints test to auth
* Update groups and ratings endpoints based on auht
* Update comments endpoints to use auth
* Update ratings endpoints to use auth
* test: Update admin titles enpoints tests to use token
* Refactor titles endpoints to be used just for admin role
* Refactor auth workflow
* Update users errors lookup
* test: Update groups enpoints tests to use token
* test: Update database migration in test setup
* Add authorization checks in groups endpoints
* test: Add minimal tests to get and update users
* Add endpoint to get user and a very simple for update fields for now
* test: Fix test to delete a user
* Update endpoint to delete a user
* test: User creation validation and deletion with auth
* Add minimal fields validation for user creation
* Add user to context and authorizarion to users endpoints
* Add username on user response
* Update mongo migration scripts
* Base auth login handler
* Apply base global auth middleware to handlers
* Update packages versions
* test: Add test to new group watched fix
* fix(groups): Setting whatchedAt date when watched is false returns 400
* test: Add more tests to group titles api
* fix(groups): Setting a title as unwatched should always clear watchedAt field in database
* test: Start test to goup titles endpoints
* test: Add setup folder to each goroup and extend test to get users from a group
* test: Endpoint to add a user to a group
* Add endpoint to add a user to a group

<a name="v0.0.2"></a>
## [v0.0.2](https://github.com/lealre/fs-mcp/compare/v0.0.1...v0.0.2) (2025-11-19)

* Update changelog for version `0.0.2`
* Add groups management for users/titles ([#2](https://github.com/lealre/fs-mcp/issues/2))
* test: update titles test
* refactor: add default response in api for messages
* refactor: separate mappers inside services
* Add small documentation on custom titles pagination
* Remove watched field from titles when adding it
* Fix group titles pagination
* Add endpoint to remove title from a group
* Add check validation to title already in a group
* Add endpoint to update the title watche state in a group
* Add endpoint to add titles to a  group
* Add endpoint to get users from a group
* Fix bson placeholder to groups and json placeholder to ratings in groups
* Add endpoint to get titles by group ID with embedded ratings
* test: create group endpoint
* Add endpoint to create a group
* test: delete users endpoint
* Add endpoint to delete user (no auth for now)
* Check duplicated username (with test)
* Add POST to create user with minimal test and auth package
* Add first version of groups collection and extend users collections field
* Add small changelog configuration

<a name="v0.0.1"></a>
## v0.0.1 (2025-11-05)

* Add more tests to title api
* Add base sctruct for testing using testcontainers
* Remove related comments when title is deleted
* Remove hash check in `scripts/backup-volume.sh`
* Refactor how handlers uses the database instance ([#1](https://github.com/lealre/fs-mcp/issues/1))
* Complete refactor to users and comments
* Complete refactor to titles and ratings
* New base struct used to inject DB on apis
* Update logic to orderBy field in titles
* Add createdAt and updatedAt fields to ratings
* Add base code to the new struct of comments
* Add small notes on setWatched endpoint and update fields from movie to titles
* Refactor ratings code
* Refactor users code
* Remove txt file and update backup script
* Refactor titles code
* Add endpoint to batch titles ratings
* Add imdb rating filed to orderby
* Add url query to order ascending/descending and add type of title in titles response
* Change watched filter to fetch from backend
* Refactor code by adding the server folder and separate the handlers by files
* Fixing the requestID isolation per request
* Adding the `watchedAt` field as option to be updated
* Setting basic pagination to titles
* Script to add new columns in title and start titles pagination
* Add script to backup and use local volume to seed mongoDB data
* Add delete titles endpoint using cascade with ratings
* Add watched field to all records and new endpoint to udpate it
* Remove sample data
* Add rating update and change rating note to float32
* Add endpoint to get the users
* Update crypto package version
* Fix bugs when adding title and getting title rating
* Small changes to make firts test of POST ratings work
* Add mongdb scripts to vscode client. Add index to ratings (titleId + userId)
* Add basic logic to ratings
* More refactor; add interfaces for rating handlers endpointsa; and add user service
* Add basic operations to add a movie from url and get the full movies list
* Start backend - Fetch movie from api and MongoDb basic crud
* Initial commit
