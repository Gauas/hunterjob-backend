const version = "000002_crawl_schedule_and_expiry";
const migrations = db.getCollection("_migrations");

if (migrations.findOne({ _id: version })) {
  print(`Migration ${version} has already been applied.`);
} else {
  db.getCollection("career_sources").createIndex(
    { enabled: 1, next_crawl_at: 1 },
    { name: "enabled_next_crawl_at" },
  );
  db.getCollection("jobs").createIndex(
    { active: 1, expired_at: 1, last_seen_at: -1 },
    { name: "active_expired_at_last_seen_at" },
  );
  migrations.insertOne({ _id: version, applied_at: new Date() });
  print(`Applied migration ${version}.`);
}
