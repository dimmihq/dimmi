-- name: ListContactCompanies :many
-- ListContactCompanies returns the live companies each of the contacts
-- belongs to, the default first and then in the order they were added.
SELECT cc.contact_id, cc.is_default, cc.created_at AS linked_at, sqlc.embed(co) FROM contact_companies AS cc
JOIN companies AS co ON co.id = cc.company_id AND co.workspace_id = cc.workspace_id
WHERE cc.workspace_id = @workspace_id AND cc.contact_id = ANY (@contact_ids::uuid[]) AND co.deleted_at IS NULL
ORDER BY cc.contact_id, cc.is_default DESC, cc.created_at, cc.company_id;

-- name: LinkContactCompany :execrows
-- LinkContactCompany adds the company to the contact and reports whether
-- it was new. The first company a contact gets becomes its default.
INSERT INTO contact_companies (workspace_id, contact_id, company_id, created_at)
VALUES (@workspace_id, @contact_id, @company_id, @now)
ON CONFLICT (contact_id, company_id) DO NOTHING;

-- name: UnlinkContactCompany :execrows
-- UnlinkContactCompany takes the company off the contact and reports
-- whether it was there. Taking off the default makes the oldest
-- remaining company the default.
DELETE FROM contact_companies
WHERE workspace_id = @workspace_id AND contact_id = @contact_id AND company_id = @company_id;

-- name: CountContactCompanies :one
-- CountContactCompanies returns how many companies the contact belongs
-- to, for the limit on companies per contact.
SELECT count(*) FROM contact_companies WHERE workspace_id = @workspace_id AND contact_id = @contact_id;

-- name: ContactCompanyIDs :many
-- ContactCompanyIDs returns the ids of the companies the contact belongs
-- to, oldest link first.
SELECT company_id FROM contact_companies
WHERE workspace_id = @workspace_id AND contact_id = @contact_id
ORDER BY created_at, company_id;

-- name: SetContactDefaultCompany :exec
-- SetContactDefaultCompany makes the company the contact's default,
-- linking it first when it is not linked yet.
UPDATE contacts SET company_id = @company_id::uuid, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND company_id IS DISTINCT FROM @company_id::uuid;

-- name: CopyContactCompanies :exec
-- CopyContactCompanies links the survivor of a merge to every company
-- the merged contact belonged to, keeping when each link was made.
INSERT INTO contact_companies (workspace_id, contact_id, company_id, created_at)
SELECT cc.workspace_id, @survivor_id::uuid, cc.company_id, cc.created_at FROM contact_companies AS cc
WHERE cc.workspace_id = @workspace_id AND cc.contact_id = @merged_id::uuid
ON CONFLICT (contact_id, company_id) DO NOTHING;

-- name: UnlinkCompanyContacts :execrows
-- UnlinkCompanyContacts takes the company off every contact, as deleting
-- the company does, and returns how many links it removed.
DELETE FROM contact_companies WHERE workspace_id = @workspace_id AND company_id = @company_id;
