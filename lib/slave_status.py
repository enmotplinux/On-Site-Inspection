# coding = utf-8
# coding = utf-8
import  pymysql
def slave_slave(host,user,password,db,port):
    conn=pymysql.connect(host=host,user=user,passwd=password,db=db,port=port)
    cur=conn.cursor()
    cur.execute('show slave status')
    alldata = cur.fetchone()
    aa=str(host)+'_'+str(port)
    new_slave={
        'id':aa,
        'status':alldata
    }
    return new_slave

