
<a name="v0.0.6"></a>
## [v0.0.6](https://github.com/lealre/fs-mcp/compare/v0.0.5...v0.0.6) (2026-02-14)

* Add cron routine to use in pi
* Small improvements in routines scripts
* Add mongo client sample format for vscode extension
* Update script to backup

<a name="v0.0.5"></a>
## [v0.0.5](https://github.com/lealre/fs-mcp/compare/v0.0.4...v0.0.5) (2026-02-08)

* Merge pull request [#7](https://github.com/lealre/fs-mcp/issues/7) from lealre/v0.0.5
* Use rclone with drive
* Merge pull request [#6](https://github.com/lealre/fs-mcp/issues/6) from lealre/v0.0.5
* Add filter option for title type when getting titles from a group
* test: New top level watchedAt logic for seasons
* Update top level information when setting a season as watched
* Command to refresh info about titles from imbd
* Add method to batch titles from api
* Add new dev migration
* Add script to make backup to dropbox
* Update `CHANGELOG.md`

### Pull Requests
* Merge pull request [#7](https://github.com/lealre/fs-mcp/issues/7) from lealre/v0.0.5
* Merge pull request [#6](https://github.com/lealre/fs-mcp/issues/6) from lealre/v0.0.5


<a name="v0.0.4"></a>
## [v0.0.4](https://github.com/lealre/fs-mcp/compare/v0.0.3...v0.0.4) (2026-02-01)

* test: Deletion of ratings for series and movies
* Add deletion of ratings for movie and series
* Merge pull request [#5](https://github.com/lealre/fs-mcp/issues/5) from lealre/v0.0.4
* Merge pull request [#4](https://github.com/lealre/fs-mcp/issues/4) from lealre/seasons
* test[seasons]: Add tests to the new comments sctruct
* feat[seasons]: Update season struct for comments
* test[seasons]: Add tests to the new ratings sctruct
* feat[seasons]: Update ratings seasons struct
* feat[seasons]: Add back ratings index for ratings
* feat[seasons]: Update imdb episodes fetch to iterate until get the last page of the episodes
* chrome[seasons]: Update dev-migrations to use new titles struct under groups
* feat[seasons]: Add missing fileds for episodes in mapper
* test[seasons]: Add tests to delete a comment from a season
* feat[seasons]: Add endpoint to delete a comment from a season
* test[seasons]: Update group tests to new tv series logic
* feat[seasons]: Pass watched/unwatched information when getting all titles from a group for series
* feat[seasons]: Update groups error handling
* test[seasons]: Update group tests to new tv series logic
* feat[seasons]: Add logic to set watched seasons form tv series
* chore[seasons]: Add migration to update groups to new struct
* test[seasons]: Update group tests to new struct
* feat[seasons]: Update titles struct under groups from an array to a map
* feat[seasons]: Start group logic to update set watched
* chore[seasons]: Remove unused js file with mongo migrations
* test/chore[seasons]: Update script to generate test fixtures
* feat[seaons]: Add more dev migrations for delet and save tv series ratings and comments
* feat[seasons]: Add small documentation on method to update a tv serie comment
* feat/test[seasons]: Add tests to update a comment and update service
* feat/test[seasons]: Remove ambiguous check in ratings test
* feat[seasons]: Update Comments struct type
* feat[seasons]: Add documentation for adding a comment for a tv series flow
* feat[seasons]: Add missing logic fixes for tests
* test[seasons]: Add tests for adding comment for seasons
* test[seasons]: Refactor for test ratings
* feat[seasons]: First cut of adding a comment to a season
* feat[seasons]: Ratings updates for tv series after tests
* feat[seasons]: Add more tests to tv series ratings updates
* feat[seasons]: First cut of season ratings update logic
* test[seasons]: Add tests to when adding tv series ratings
* test[seasons]: Fix fixture titles structs for eppisodes and seasons
* feat[seasons]: Update structs of episodes and seasons for titles and ratings flow
* feat[seasons]: Update seasons information for ratings that already exists
* test[seasons]: Add script to create the test fixtures for tests
* test[seasons]: Split ratings tests for adding and updating
* feat[seasons]: Add seasons to rating level
* feat[seasons]: Add dev-migrations to validate changes in development
* feat[seasons]: Fetch information about season/episodes when title type is series/mini-serie
* test/refactor: Split group tests in different methods
* test: Add more tests for groups creation - empty or duplicated names for same user
* Add api error handling for groups empty or duplicated names
* Add new index do database - unique group name/ownerId combination non-null
* test: Update test to delet a user to actually try to delete another user that exists
* test: Update endpoint to user getting own information
* Add endpoint to get current user information based on the token
* Use common logic to database index creation for both cli migrationa and tests
* Add indexes logic to mongodb internal package
* test: Sync database migration with cli and add time.sleep to comments before test updating
* Remove old dockerhub push script
* Add flag to accept exisitng volume in `docker-compose.yaml`
* Update script to push image to dockerhub
* Update docker setup to push to hub
* fix: database filter was returning just name of the users from a group
* fix: groups with no titles was returning all titles from collections
* Update cli to delte/reset indexes and update indexes names
* Update `README.md`
* Update `env.example`
* Update `Dockerfile` to use the new database cli
* Add option to create a superuser in databse cli
* Add `Dockerfile` to push images to docker hub
* Update `README.md` with new repo name and how to dump/retore mongo data
* Add go cmd to create the database indexes
* Break `MONGO_URI` in its own variables
* Change how volumes are managed in docker and create scripts to dump/restore data
* Update changelog for version 0.0.3

### Pull Requests
* Merge pull request [#5](https://github.com/lealre/fs-mcp/issues/5) from lealre/v0.0.4
* Merge pull request [#4](https://github.com/lealre/fs-mcp/issues/4) from lealre/seasons


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
