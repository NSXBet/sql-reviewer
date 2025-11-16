-- PostgreSQL Syntax Error Examples
-- This file demonstrates how syntax errors are converted to actionable advice
-- with line/column position information (error code 201)

-- Example 1: Missing table name in CREATE TABLE
-- Error: "no viable alternative at input 'CREATE TABLE;'"
-- Position: line 7, column 12
CREATE TABLE;

-- Example 2: Invalid INSERT statement (missing INTO)
-- Error: "missing 'INTO' at 'users'"
-- Position: line 13, column 7
INSERT users VALUES (1, 'John');

-- Example 3: Missing table name in SELECT FROM
-- Error: "mismatched input 'WHERE' expecting ..."
-- Position: line 19, column 14
SELECT * FROM WHERE id = 1;

-- Example 4: Incomplete ALTER TABLE statement
-- Error: "no viable alternative at input 'ALTER TABLE ADD;'"
-- Position: line 25, column 15
ALTER TABLE ADD;

-- Example 5: Incomplete CREATE INDEX statement
-- Error: "mismatched input ';' expecting ..."
-- Position: line 31, column 12
CREATE INDEX;

-- Example 6: Malformed column definition
-- Error: Syntax error with position
CREATE TABLE users (
    id INT,
    name INVALID TYPE,
    email VARCHAR(255)
);

-- Example 7: Invalid constraint syntax
-- Error: Syntax error with position
ALTER TABLE users ADD CONSTRAINT INVALID;

-- Example 8: Incomplete DROP TABLE
-- Error: Syntax error with position
DROP TABLE;

-- Valid SQL for comparison (these will pass syntax checks)
CREATE TABLE valid_table (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO valid_table (name) VALUES ('Test User');

SELECT id, name, created_at FROM valid_table WHERE id = 1;

ALTER TABLE valid_table ADD COLUMN email VARCHAR(255);

CREATE INDEX idx_valid_table_name ON valid_table(name);

DROP TABLE valid_table;
