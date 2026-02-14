// Create unique index for users collection (case-insensitive)
db("name")
  .collection("users")
  .createIndex(
    { email: 1 },
    {
      unique: true,
      collation: { locale: "en", strength: 2 },
      partialFilterExpression: { email: { $exists: true } },
    }
  )
  .then((result) => {
    console.log("✅ Unique index created:", result);
  })
  .catch((err) => {
    console.error("❌ Error creating index:", err);
  });

// Create unique index for users collection (case-insensitive)
db("name")
  .collection("users")
  .createIndex(
    { username: 1 },
    {
      unique: true,
      collation: { locale: "en", strength: 2 },
      partialFilterExpression: { username: { $exists: true } },
    }
  )
  .then((result) => {
    console.log("✅ Unique index created:", result);
  })
  .catch((err) => {
    console.error("❌ Error creating index:", err);
  });

// Create unique index for ratings collection
db("name")
  .collection("ratings")
  .createIndex({ userId: 1, titleId: 1 }, { unique: true })
  .then((result) => {
    console.log(`✅ Unique index created: ${result}`);
  })
  .catch((err) => {
    console.error("❌ Error creating unique index:", err);
  });

// Create unique index for comments collection
db("name")
  .collection("comments")
  .createIndex({ userId: 1, titleId: 1 }, { unique: true })
  .then((result) => {
    console.log(`✅ Unique index created: ${result}`);
  })
  .catch((err) => {
    console.error("❌ Error creating unique index:", err);
  });