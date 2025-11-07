-- PostgreSQL Test File

-- Good table with all requirements
CREATE TABLE public.users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL,
    email VARCHAR(100) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create index concurrently
CREATE INDEX CONCURRENTLY idx_users_email ON public.users(email);

-- Good INSERT with columns specified
INSERT INTO public.users (username, email) VALUES ('john_doe', 'john@example.com');

-- Good SELECT with WHERE
SELECT id, username, email FROM public.users WHERE id = 1;

-- Good UPDATE with WHERE
UPDATE public.users SET username = 'jane_doe' WHERE id = 1;

-- Good DELETE with WHERE
DELETE FROM public.users WHERE id = 1;
