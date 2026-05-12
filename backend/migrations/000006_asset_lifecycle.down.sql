alter table assets drop column if exists deleted_at;
alter table assets drop column if exists status;
drop type if exists asset_status;
