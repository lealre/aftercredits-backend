// Add users
// db('brunan').collection('users').insertMany([
//     { _id: "68e67788956f936302a2a778", name: 'Renan' },
//     { _id: "68e67788956f936302a2a779", name: 'Bruna' }
//   ]);

// Create unique index for ratings collection
db("brunan")
  .collection("ratings")
  .createIndex({ userId: 1, titleId: 1 }, { unique: true });

  // Create unique index for comments collection
db("brunan")
.collection("comments")
.createIndex({ userId: 1, titleId: 1 }, { unique: true })
.then((result) => {
  console.log(`✅ Unique index created: ${result}`);
})
.catch((err) => {
  console.error("❌ Error creating unique index:", err);
});

// Delete a title (example)
db("brunan")
  .collection("titles")
  .deleteOne({ _id: "tt0117060" })
  .then((result) => {
    console.log(`${result.deletedCount} document deleted`);
  })
  .catch((err) => {
    console.error("Error deleting title:", err);
  });

// Add new fields to titles
db("brunan")
  .collection("titles")
  .updateMany({}, { $set: { addedAt: new Date(), updatedAt: new Date(), watchedAt: null } });