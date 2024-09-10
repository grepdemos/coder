CREATE TABLE "keys" (
    "feature" text NOT NULL,
    "sequence" integer NOT NULL,
    "secret" text NULL,
    "starts_at" timestamptz NOT NULL,
    "deletes_at" timestamptz NULL,
    PRIMARY KEY ("feature", "sequence")
);
