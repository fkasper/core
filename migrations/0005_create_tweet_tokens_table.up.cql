CREATE TABLE shared_tweet_tokens (
	id uuid,
	active boolean,
	avatar_id uuid,
	added_at timestamp,
	PRIMARY KEY (id)
) WITH comment = 'Table for letting external People tweet on SITREP';
