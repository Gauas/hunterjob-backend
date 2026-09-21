// This migration is intentionally idempotent.  A partially completed run can
// safely be retried; the migration record is written only after all indexes
// have been created successfully.
const version = "000001_initial_indexes";
const migrations = db.getCollection("_migrations");

if (migrations.findOne({ _id: version })) {
  print(`Migration ${version} has already been applied.`);
} else {
  db.getCollection("jobs").createIndexes([
    { key: { original_url: 1 }, name: "original_url_unique", unique: true },
    { key: { fingerprint: 1 }, name: "fingerprint" },
    { key: { active: 1, last_seen_at: -1 }, name: "active_last_seen_at" },
  ]);
  db.getCollection("companies").createIndex(
    { slug: 1 },
    { name: "slug_unique", unique: true },
  );
  db.getCollection("career_sources").createIndex(
    { career_url: 1 },
    { name: "career_url_unique", unique: true },
  );
  db.getCollection("click_events").createIndex(
    { job_id: 1, created_at: -1 },
    { name: "job_id_created_at" },
  );

  migrations.insertOne({
    _id: version,
    applied_at: new Date(),
  });
  print(`Applied migration ${version}.`);
}
