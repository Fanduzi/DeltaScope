ALTER POLICY users_select ON users USING (user_id = current_user);
