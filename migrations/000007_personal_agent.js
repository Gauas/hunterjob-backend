const version = "000007_personal_agent";
const migrations = db.getCollection("_migrations");

if (migrations.findOne({ _id: version })) {
  print(`Migration ${version} has already been applied.`);
} else {
  db.getCollection("search_preferences").createIndex({ user_id: 1 }, { name: "user_unique", unique: true });
  db.getCollection("job_matches").createIndex({ user_id: 1, job_id: 1 }, { name: "user_job_unique", unique: true });
  db.getCollection("job_matches").createIndex({ user_id: 1, dedupe_key: 1 }, { name: "user_dedupe_unique", unique: true, partialFilterExpression: { dedupe_key: { $type: "string" } } });
  db.getCollection("job_matches").createIndex({ user_id: 1, matched_at: -1 }, { name: "user_recent_matches" });
  db.getCollection("job_matches").createIndex({ user_id: 1, status: 1, matched_at: -1 }, { name: "user_status_recent_matches" });
  db.getCollection("chat_connections").createIndex({ user_id: 1, provider: 1 }, { name: "user_provider_unique", unique: true });
  db.getCollection("notification_deliveries").createIndex({ user_id: 1, created_at: -1 }, { name: "user_recent_deliveries" });
  migrations.insertOne({ _id: version, applied_at: new Date() });
  print(`Applied migration ${version}.`);
}
