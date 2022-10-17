# sql

A simple sql wrapper provides convenient CRUD operations for struct objects.
 
## Mapping struct fields to sql columns
1. Column name is converted from field name with CamelToSnake pattern by default
1. Custom column name can be declared with `sql` tag 
1. `primary key`, `auto_increment` are supported in db tag
1. Use \`sql:"-"\` to ignore fields
1. Column must be field which can be exported

        type Product struct {
    	    ID        int `sql:"primary key,auto_increment"`
    	    Name      string
    	    Price     float32
    	    Text      string `sql:"txt"`
    	    UpdatedAt int64
    	    
    	    Ext any `sql:"-"'
        }
        
## Table Name
1. Default table name is the plural form of struct name. 

    `type Product struct`'s table name is `products`  
    `type User struct`'s table name is `users`
1. Custom table name is provided by `TableName` method

        type User struct {
            ID int `sql:"primary key"`
            Name string
        }
        
        func (u *User) TableName() string {
            return "users" + fmt.Sprint(u.id%100)
        }

## Open database

    	db, err := NewDB("mysql", "dbuser:dbpassword@tcp(localhost:3306)/dbname")
    	...

## Insert

        p := &Product{
            Name:      "apple",
            Price:     0.1,
            Text:      "nice",
            UpdatedAt: time.Now().Unix(),
        }
        db.Insert(p)
        
## Update

        p.Price = 0.2
        db.Update(p)
        
## Save
Save is supported by mysql and sqlite3 drivers. It will insert the record if it does't exist, otherwise update the record.
       
        p.Price = 0.3
        db.Save(p)
        
        p = &Product{
            Name:      "apple",
            Price:     0.1,
            Text:      "nice",
            UpdatedAt: time.Now().Unix(),
        }
        db.Save(p)
        
## Select

        var products []*Product
        //Select all products
        db.Select(&products)
        
        //Select products whose price is less than 0.2
        db.Select(&products, "price<?", 0.2)
        
## SelectOne

        var p1 *Product
        db.SelectOne(&p1)
     
        var p2 Product
        db.SelectOne(&p2, "id=?", 3)
        
## Specify table name explicitly

        db.Table("products").Insert(p)
        
## Transaction
        
        tx, err := db.Begin()
        tx.Insert(p1)
        tx.Insert(p2)
        tx.Table("products").Insert(p3)
        tx.Commit()
        
## Support embedded struct
        
        type Product struct {
            ID        int `sql:"primary key,auto_increment"`
            Name      string
            Price     float32
            Text      string `sql:"txt"`
            UpdatedAt int64
        }
        
        type ProductDetail struct {
            Product
            Detail string
        }

## Support json

        type Coordinate struct {
            Lng float
            Lat float
        }
        
        type Topic struct {
            ID          int64
            Title       string
            Location    *Coordinate `sql:"json"`
        }