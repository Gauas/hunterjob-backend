const version = "000006_public_career_pages";
const migrations = db.getCollection("_migrations");

if (migrations.findOne({ _id: version })) {
  print(`Migration ${version} has already been applied.`);
} else {
  db.getCollection("career_sources").updateMany(
    { career_url: /^https:\/\/ots\.one-line\.com\/api\/jobs(?:\?|$)/ },
    {
      $set: {
        career_page_url: "https://ots.one-line.com/vi/careers",
        updated_at: new Date(),
      },
      $unset: {
        job_url_template: "",
        apply_url_template: "",
      },
    },
  );
  migrations.insertOne({ _id: version, applied_at: new Date() });
  print(`Applied migration ${version}.`);
}
