const version = "000005_job_url_templates";
const migrations = db.getCollection("_migrations");

if (migrations.findOne({ _id: version })) {
  print(`Migration ${version} has already been applied.`);
} else {
  const otsJobURL = "https://ots.one-line.com/vi/careers/{slug}-{id}";
  db.getCollection("career_sources").updateMany(
    { career_url: /^https:\/\/ots\.one-line\.com\/api\/jobs(?:\?|$)/ },
    {
      $set: {
        job_url_template: otsJobURL,
        apply_url_template: otsJobURL,
        updated_at: new Date(),
      },
    },
  );
  migrations.insertOne({ _id: version, applied_at: new Date() });
  print(`Applied migration ${version}.`);
}
