CREATE TRANSFORM FOR jsonb LANGUAGE plpython3u (FROM SQL WITH FUNCTION jsonb_to_plpython(jsonb), TO SQL WITH FUNCTION plpython_to_jsonb(internal))
