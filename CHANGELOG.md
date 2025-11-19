
<a name="v0.0.2"></a>
## [v0.0.2](https://github.com/lealre/fs-mcp/compare/v0.0.1...v0.0.2) (2025-11-18)

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
