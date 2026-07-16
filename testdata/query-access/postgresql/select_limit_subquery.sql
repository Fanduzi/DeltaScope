SELECT id FROM public.users LIMIT (SELECT max_val FROM public.config)
