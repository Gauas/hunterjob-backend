const version = "000008_telegram_chat_unique";
const migrations = db.getCollection("_migrations");

if (migrations.findOne({ _id: version })) {
  print(`Migration ${version} has already been applied.`);
} else {
  db.getCollection("chat_connections").createIndex(
    { provider: 1, external_user_id: 1 },
    {
      name: "telegram_chat_unique",
      unique: true,
      partialFilterExpression: {
        provider: "telegram",
        status: "connected",
        external_user_id: { $type: "string" },
      },
    },
  );
  migrations.insertOne({ _id: version, applied_at: new Date() });
  print(`Applied migration ${version}.`);
}
