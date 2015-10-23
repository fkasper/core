CREATE TABLE IF NOT EXISTS access_users (
	id uuid,
	email text,
	hashed_password text,
	password_salt text,
	is_active boolean,
	encryption_keys MAP<text, text>,
	meta_storage MAP<text, text>,
	real_name text,
	sign_in_count int,
	activated_at timestamp,
	activated_by uuid,
  twittername text,
	PRIMARY KEY (id, email)
) WITH comment = '';
