const version = "000004_drop_legacy_original_url_index";
const migrations = db.getCollection("_migrations");

if (migrations.findOne({ _id: version })) {
  print(`Migration ${version} has already been applied.`);
} else {
  const jobs = db.getCollection("jobs");
  for (const index of jobs.getIndexes()) {
    const keys = Object.keys(index.key);
    if (index.unique && keys.length === 1 && index.key.original_url === 1) {
      print(`Dropping legacy index ${index.name}.`);
      jobs.dropIndex(index.name);
    }
  }
  jobs.createIndex(
    { source_id: 1, source_job_id: 1 },
    {
      name: "source_job_id_unique",
      unique: true,
      partialFilterExpression: { source_job_id: { $type: "string" } },
    },
  );
  migrations.insertOne({ _id: version, applied_at: new Date() });
  print(`Applied migration ${version}.`);
}
