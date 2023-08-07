import tempfile
import copy
import os
def extend(params_old):
    params=[]
    params.append( params_old[0] )
    for idx in range(len(params_old)-1):
        params.append( (params_old[idx+1]+params_old[idx])//2 )
        params.append( params_old[idx+1] )
    return params

class RunBench:
    def __init__(self,command,result_name,final_name,binary_dir):
        self.command = command
        self.result_name = result_name
        self.final_name = final_name
        self.binary_dir = binary_dir
    def RunCommand(self):

        tmpdir_real=tempfile.TemporaryDirectory()
        tmpdir = tmpdir_real.name
        ###first copy all binary to this temperary directory
        print(tmpdir) 
        copy_binary_command = "cp -r %s/* %s"%(self.binary_dir,tmpdir)
        ##change workdir to tmpdir
        os.chdir( tmpdir )
        
        os.system(copy_binary_command)

        ##run command
        os.system(self.command)
        ##copy final result to gpudata directory
        copy_result_command = "mv %s %s"%(self.result_name,self.final_name) 
        #print(copy_result_command)
        
        os.system(copy_result_command)
        tmpdir_real.cleanup()
        ## print(tmpdir)

hash2filename = dict()
def add_bench(bench,binary,parameter, pattern_para,pattern,v=None):
    list1= [ binary , parameter , pattern_para]
    #print(list1)
    cmd1 = " ".join( list1)
    
    cmdparas = remove_special_charactor(binary)
    pattern_paras=remove_special_charactor(pattern_para)

    oriname = results_name[pattern]
    oriname_rep=copy.copy(oriname).replace(".json","")
    if v == None:
        v = "0"
    if v=="0":
        final_name = oriname_rep +"_" + cmdparas+"_" + pattern_paras + ".json"
    else:
        final_name = oriname_rep +"_" + cmdparas+"_" + pattern_paras+"_v"+v + ".json"

    print(final_name)
    hash2filename[hash( cmd1 )] = final_name
#    os.system(cmd1)
    return final_name,cmd1
results_name = dict()
results_name["full"] = "result.json"
results_name["inscount"] = "insnums.json"
results_name["wgsampled"] = "sampled_result.json"
results_name["branchsampled"] = "sampled_result.json"
results_name["mixedsampled"] = "sampled_result.json"
results_name["ipcsampled"] = "ipc_sampled_result.json"

results_name["analysis"] = "bbv_feature.json"
def remove_special_charactor(pattern_para):
    paras=copy.copy(pattern_para).replace(" ","_")
    paras=paras.replace("-","_")
    paras=paras.replace(".","_")
    paras=paras.replace("=","_")
    paras=paras.replace("/","_")
    return paras

