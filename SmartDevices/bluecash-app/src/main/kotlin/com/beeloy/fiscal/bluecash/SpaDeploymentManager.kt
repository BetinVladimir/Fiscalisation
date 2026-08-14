package com.beeloy.fiscal.bluecash

import android.content.Context
import org.json.JSONObject
import java.io.File
import java.net.URL
import java.security.KeyFactory
import java.security.MessageDigest
import java.security.Signature
import java.security.spec.X509EncodedKeySpec
import java.util.Base64

data class SpaDeploymentState(val applicationId:String="com.beeloy.miniposweb",val version:String="none",val buildId:String="none",val state:String="FAILED",val errorCode:String?=null)

class SpaDeploymentManager(private val context:Context,private val descriptorUrl:String?,private val expectedKid:String?,publicKeyDerBase64:String?) {
    private val root=File(context.filesDir,"spa").apply{mkdirs()}
    private val preferences=context.getSharedPreferences("beeloy-spa-deployment",Context.MODE_PRIVATE)
    private val publicKey=publicKeyDerBase64?.takeIf{it.isNotBlank()}?.let{KeyFactory.getInstance("Ed25519").generatePublic(X509EncodedKeySpec(Base64.getDecoder().decode(it)))}
    @Volatile private var current=loadState()
    fun state()=current
    fun activeRoot():File=File(root,preferences.getString("active_slot","slot-a")!!)
    fun checkAndActivate():SpaDeploymentState=runCatching{update()}.getOrElse{failure->current=SpaDeploymentState(errorCode=failure.message?:"DEPLOYMENT_FAILED");current}
    private fun update():SpaDeploymentState {
        require(!descriptorUrl.isNullOrBlank()&&descriptorUrl.startsWith("https://")&&publicKey!=null&&!expectedKid.isNullOrBlank()){"DEPLOYMENT_TRUST_UNAVAILABLE"}
        val descriptorText=get(URL(descriptorUrl),1_048_576)
        val descriptor=JSONObject(String(descriptorText))
        val signatureObject=descriptor.getJSONObject("signature")
        require(signatureObject.getString("kid")==expectedKid&&signatureObject.getString("alg")=="Ed25519"){"DEPLOYMENT_SIGNATURE_HEADER"}
        val signature=Base64.getUrlDecoder().decode(signatureObject.getString("value"));descriptor.remove("signature")
        val verifier=Signature.getInstance("Ed25519");verifier.initVerify(publicKey);verifier.update(descriptor.toString().toByteArray());require(verifier.verify(signature)){"DEPLOYMENT_SIGNATURE_INVALID"}
        require(descriptor.getInt("schema_version")==1&&descriptor.getString("application_id")=="com.beeloy.miniposweb"&&descriptor.getString("minimum_adapter_api")<="2026-08-14"){"DEPLOYMENT_INCOMPATIBLE"}
        val version=descriptor.getString("version");val buildId=descriptor.getString("build_id");val old=current
        if(old.state=="ACTIVE"&&old.buildId==buildId)return old
        require(old.version=="none"||compareVersions(version,old.version)>=0){"DEPLOYMENT_ROLLBACK_FORBIDDEN"}
        val inactive=if(preferences.getString("active_slot","slot-a")=="slot-a")"slot-b" else "slot-a";val staging=File(root,"$inactive.tmp")
        staging.deleteRecursively();require(staging.mkdirs()){"DEPLOYMENT_STORAGE"};val files=descriptor.getJSONArray("files");require(files.length() in 1..512){"DEPLOYMENT_FILES"};var total=0L
        for(index in 0 until files.length()){val item=files.getJSONObject(index);val path=item.getString("path");require(validPath(path)){"DEPLOYMENT_PATH"};val size=item.getLong("size");require(size in 0..8_388_608){"DEPLOYMENT_SIZE"};total+=size;require(total<=32L*1024*1024){"DEPLOYMENT_TOTAL_SIZE"};val bytes=get(URL(URL(descriptorUrl),"/$path"),size.toInt()+1);require(bytes.size.toLong()==size&&sha256(bytes)==item.getString("sha256")){"DEPLOYMENT_DIGEST"};val destination=File(staging,path);require(destination.canonicalPath.startsWith(staging.canonicalPath+File.separator)){"DEPLOYMENT_PATH"};destination.parentFile?.mkdirs();destination.outputStream().use{stream->stream.write(bytes);stream.fd.sync()}}
        require(File(staging,descriptor.getString("entrypoint")).isFile){"DEPLOYMENT_ENTRYPOINT"};File(staging,".verified").writeText(buildId);val target=File(root,inactive);target.deleteRecursively();require(staging.renameTo(target)){"DEPLOYMENT_ATOMIC_SWITCH"}
        preferences.edit().putString("active_slot",inactive).putString("version",version).putString("build_id",buildId).apply();current=SpaDeploymentState(version=version,buildId=buildId,state="ACTIVE");return current
    }
    private fun loadState():SpaDeploymentState{val version=preferences.getString("version",null)?:return SpaDeploymentState();val build=preferences.getString("build_id","none")!!;return if(File(activeRoot(),".verified").readTextOrNull()==build)SpaDeploymentState(version=version,buildId=build,state="ACTIVE")else SpaDeploymentState(errorCode="DEPLOYMENT_MARKER_INVALID")}
    private fun File.readTextOrNull()=runCatching{readText()}.getOrNull()
    private fun validPath(path:String)=path.matches(Regex("[A-Za-z0-9._/-]{1,240}"))&&!path.startsWith('/')&&path.split('/').none{it==".."||it.isBlank()}
    private fun get(url:URL,limit:Int):ByteArray{val connection=url.openConnection() as java.net.HttpURLConnection;require(connection is javax.net.ssl.HttpsURLConnection){"DEPLOYMENT_HTTPS_REQUIRED"};connection.connectTimeout=10_000;connection.readTimeout=30_000;connection.instanceFollowRedirects=false;connection.inputStream.use{input->val out=java.io.ByteArrayOutputStream();val buffer=ByteArray(8192);while(true){val n=input.read(buffer);if(n<0)break;require(out.size()+n<=limit){"DEPLOYMENT_DOWNLOAD_LIMIT"};out.write(buffer,0,n)};return out.toByteArray()}}
    private fun sha256(value:ByteArray)=MessageDigest.getInstance("SHA-256").digest(value).joinToString(""){"%02x".format(it)}
    private fun compareVersions(left:String,right:String):Int{val a=left.substringBefore('-').split('.').map{it.toIntOrNull()?:0};val b=right.substringBefore('-').split('.').map{it.toIntOrNull()?:0};for(index in 0 until maxOf(a.size,b.size)){val result=(a.getOrElse(index){0}).compareTo(b.getOrElse(index){0});if(result!=0)return result};return 0}
}
