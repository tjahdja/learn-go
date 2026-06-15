-- 1. Add the new user_id column to the todos table
ALTER TABLE todos ADD COLUMN user_id INT;

-- 2. Establish a Foreign Key constraint linking it back to the users table
ALTER TABLE todos
ADD CONSTRAINT fk_todo_user
FOREIGN KEY (user_id)
REFERENCES users(id)
ON DELETE CASCADE;
