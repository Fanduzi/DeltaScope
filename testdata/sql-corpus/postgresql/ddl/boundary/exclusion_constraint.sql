CREATE TABLE bookings (
  room_id int,
  during tsrange,
  EXCLUDE USING gist (room_id WITH =, during WITH &&)
);
