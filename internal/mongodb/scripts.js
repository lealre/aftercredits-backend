// INSERT USERS
db("brunan")
  .collection("users")
  .insertMany([
    { _id: "68e67788956f936302a2a778", name: "Renan" },
    { _id: "68e67788956f936302a2a779", name: "Bruna" },
  ]);

// ADD INDEX TO RATINGS COLLECTION
db("brunan")
  .collection("ratings")
  .createIndex({ userId: 1, titleId: 1 }, { unique: true });
