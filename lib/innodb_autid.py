# coding = utf-8
import  pymysql
def auth_status(host,user,password,db,port):
    conn=pymysql.connect(host=host,user=user,passwd=password,db=db,port=port)
    cur=conn.cursor()
    cur.execute('SELECT table_schema,table_name,engine,Auto_increment FROM information_schema.tables where TABLE_SCHEMA  '
                'not in ("information_schema" ,"performance_schema","mysql","sys")')
    alldata = cur.fetchall()

    aa = str(host) + '_' + str(port)
    new_slave = {
        'id': aa,
        'status': alldata
    }
    return new_slave



