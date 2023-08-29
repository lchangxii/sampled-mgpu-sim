#import imp
import pandas as pd
import numpy as np
import os
import sys


def export_excel( excel_name, datas, columns ):
 #   if os.path.isfile(excel_name):
#        os.remove(excel_name)
    first_one = True

    for sheetname, data in datas.items():
        if first_one:
            export_excel_sheet(excel_name,sheetname, data,columns ,False)
            first_one = False
        else:       
            export_excel_sheet(excel_name,sheetname, data,columns )

def export_excel_sheet( excel_name,sheetname,datas, columns , append=True):

    #df = pd.DataFrame(list(zip(technologies,fee,duration,discount)), columns=columns)
#    df = pd.DataFrame(list(zip(*datas)), columns=columns)
    df = pd.DataFrame( datas, columns=columns)
#    print(df)
#    print(sheetname)
    if append:
        with pd.ExcelWriter( excel_name,mode='a') as writer:
            df.to_excel( writer ,sheet_name=sheetname)
    else:
        df.to_excel( excel_name,sheet_name=sheetname)

    #df.to_excel('Courses.xlsx',sheet_name="name2")

