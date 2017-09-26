# coding = utf-8
import sys,os,json
from pymongo import MongoClient
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))



def conf_mo(host,user,password,db,port):

    a_json = json.load(open('../conf/conf','r'))
    conn = MongoClient(a_json['host'], int(a_json['port']))
    db_auth=conn.abc
    db_auth.authenticate(a_json['user'], a_json['passwd'])
    db_auth.user.insert({'host':host,'user':user,'password':password ,'db':db,'port':port})

host_host=input('host:')
user_user=input('user:')
paswd=input('password:')
db_db='mysql'
port_port=str(input('port:'))

conf_mo(host_host,user_user,paswd,db_db,port_port)


