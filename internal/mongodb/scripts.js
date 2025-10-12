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

// ADD NEW FIELD TO ALL TITLES
db("brunan")
  .collection("titles")
  .updateMany({}, { $set: { watched: false } })
  .then((result) => {
    console.log(`${result.modifiedCount} documents updated`);
  })
  .catch((err) => {
    console.error("Error updating titles:", err);
  });

// DELETE TITLE
db("brunan")
  .collection("titles")
  .deleteOne({ _id: "tt0117060" })
  .then((result) => {
    console.log(`${result.deletedCount} document deleted`);
  })
  .catch((err) => {
    console.error("Error deleting title:", err);
  });
