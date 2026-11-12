-- T26-SERVER-ALTER-NATIVE-ORACLE-01 — account + seed setup (MySQL 8.x).
-- Run as root: mysql --force -v < setup_8x.sql
-- All values are sentinels; no real host/user/password material.

drop user if exists na_alt@'%';
create user na_alt@'%' identified by 'probe';
grant SUPER on *.* to na_alt@'%';                         -- write-priv only, no catalog SELECT

drop user if exists na_altsel@'%';
create user na_altsel@'%' identified by 'probe';
grant SUPER on *.* to na_altsel@'%';
grant SELECT on mysql.servers to na_altsel@'%';           -- write-priv + table-level read

drop user if exists na_selonly@'%';
create user na_selonly@'%' identified by 'probe';
grant SELECT on mysql.servers to na_selonly@'%';          -- read only, no write-priv

drop user if exists na_usage@'%';
create user na_usage@'%' identified by 'probe';           -- connect only

drop user if exists na_roleuser@'%';
create user na_roleuser@'%' identified by 'probe';
drop role if exists na_fedrole;
create role na_fedrole;
grant SUPER on *.* to na_fedrole;
grant na_fedrole to na_roleuser@'%';
alter user na_roleuser@'%' default role na_fedrole;       -- activation still governed by role state

-- seed servers: one dedicated name per case family
drop server if exists nao_a1;
drop server if exists nao_b1;
drop server if exists nao_e1;
drop server if exists nao_e2;
drop server if exists nao_d1;
drop server if exists nao_d2;
drop server if exists nao_r1;
drop server if exists nao_r2;
drop server if exists nao_merge;
drop server if exists nao_g2;
drop server if exists nao_g3u;
drop server if exists nao_g3h;
drop server if exists nao_g3p;
drop server if exists nao_p0;
drop server if exists nao_p1;
drop server if exists nao_p9;
drop server if exists nao_p10;
drop server if exists nao_pn1;
drop server if exists nao_pq;
drop server if exists nao_pov;
drop server if exists MixedCase;
drop server if exists nao_h64_abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcd;
drop server if exists nao_h65_abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcde;
create server nao_a1 foreign data wrapper mysql options (USER 'seed_u', PASSWORD 'seed_p', HOST 'seed_h', DATABASE 'seed_d', PORT 3360, SOCKET 'seed_s', OWNER 'seed_o');
create server nao_b1 foreign data wrapper mysql options (USER 'seed_u');
create server nao_e1 foreign data wrapper mysql options (USER 'seed_u');
create server nao_e2 foreign data wrapper mysql options (USER 'seed_u');
create server nao_d1 foreign data wrapper mysql options (USER 'seed_u');
create server nao_d2 foreign data wrapper mysql options (USER 'seed_u');
create server nao_r1 foreign data wrapper mysql options (USER 'seed_u');
create server nao_r2 foreign data wrapper mysql options (USER 'seed_u');
create server nao_merge foreign data wrapper mysql options (HOST 'm_host', DATABASE 'm_db', USER 'm_user', PASSWORD 'm_pass', PORT 3361, SOCKET 'm_sock', OWNER 'm_owner');
create server nao_g2 foreign data wrapper mysql options (USER 'g2_seed', HOST 'g2_h');
create server nao_g3u foreign data wrapper mysql options (USER 'g3_seed', HOST 'g3_h');
create server nao_g3h foreign data wrapper mysql options (USER 'g3_seed', HOST 'g3_h');
create server nao_g3p foreign data wrapper mysql options (USER 'g3_seed', PASSWORD 'g3_old');
create server nao_p0 foreign data wrapper mysql options (USER 'p_seed');
create server nao_p1 foreign data wrapper mysql options (USER 'p_seed');
create server nao_p9 foreign data wrapper mysql options (USER 'p_seed');
create server nao_p10 foreign data wrapper mysql options (USER 'p_seed');
create server nao_pn1 foreign data wrapper mysql options (USER 'p_seed');
create server nao_pq foreign data wrapper mysql options (USER 'p_seed');
create server nao_pov foreign data wrapper mysql options (USER 'p_seed');
create server MixedCase foreign data wrapper mysql options (USER 'mc_seed');
-- exactly 64 chars: 'nao_h64_'(8) + 56 = 64
create server nao_h64_abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcd foreign data wrapper mysql options (USER 'h64_seed');
-- exactly 65 chars: boundary probe — does CREATE truncate or reject?
create server nao_h65_abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcde foreign data wrapper mysql options (USER 'h65_seed');
select Server_name, length(Server_name) as name_bytes, char_length(Server_name) as name_chars from mysql.servers where Server_name like 'nao_%' or Server_name like '%ixedCase%' order by Server_name;
show grants for na_alt@'%';
show grants for na_altsel@'%';
show grants for na_selonly@'%';
show grants for na_usage@'%';
show grants for na_roleuser@'%';
