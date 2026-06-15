-- 1. Drop the foreign key constraint first
ALTER TABLE todos DROP CONSTRAINT IF EXISTS fk_todo_user;

-- 2. Remove the user_id column from the todos table
ALTER TABLE todos DROP COLUMN IF EXISTS user_id;
