CREATE TABLE t (
  e_upper ENUM('Ready','Closed'),
  e_lower ENUM('ready','closed'),
  e_order ENUM('Closed','Ready'),
  s_upper SET('Read','Write'),
  s_lower SET('read','write'),
  s_order SET('Write','Read')
);
