ALTER TABLE services
ADD CONSTRAINT uq_services_user_url
UNIQUE (user_id, url);