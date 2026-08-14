#include "command_queue.h"
#include "freertos/FreeRTOS.h"
#include "freertos/queue.h"
#include "freertos/task.h"
#include <cstdlib>
#include <cstring>
namespace beefiscal::idf{
struct Owned{uint8_t*data;size_t size;Ingress ingress;SemaphoreHandle_t done;esp_err_t*result;};
struct CommandQueue::Impl{QueueHandle_t queue{};CommandSink sink{};void*context{};};
static void worker(void*raw){auto*self=static_cast<CommandQueue::Impl*>(raw);Owned item{};
  for(;;)if(xQueueReceive(self->queue,&item,portMAX_DELAY)==pdTRUE){
    const esp_err_t result=self->sink({item.data,item.size,item.ingress},self->context);if(item.done&&item.result){*item.result=result;xSemaphoreGive(item.done);}else free(item.data);}}
esp_err_t CommandQueue::start(CommandSink sink,void*context,size_t depth){
  if(impl_||!sink||depth<1||depth>32)return ESP_ERR_INVALID_ARG;
  impl_=new Impl;
  impl_->sink=sink;impl_->context=context;impl_->queue=xQueueCreate(depth,sizeof(Owned));
  if(!impl_->queue)return ESP_ERR_NO_MEM;
  return xTaskCreate(worker,"intent-worker",8192,impl_,10,nullptr)==pdPASS?ESP_OK:ESP_ERR_NO_MEM;}
esp_err_t CommandQueue::enqueue(const CommandView&v){if(!impl_||!v.data||!v.size||v.size>8192)return ESP_ERR_INVALID_ARG;
  auto*copy=static_cast<uint8_t*>(malloc(v.size));if(!copy)return ESP_ERR_NO_MEM;memcpy(copy,v.data,v.size);
  Owned item{copy,v.size,v.ingress,nullptr,nullptr};if(xQueueSend(impl_->queue,&item,0)!=pdTRUE){free(copy);return ESP_ERR_NO_MEM;}return ESP_OK;}
esp_err_t CommandQueue::enqueue_and_wait(const CommandView&v,uint32_t timeout){if(!impl_||!v.data||!v.size||v.size>8192||timeout<1)return ESP_ERR_INVALID_ARG;auto*copy=static_cast<uint8_t*>(malloc(v.size));if(!copy)return ESP_ERR_NO_MEM;memcpy(copy,v.data,v.size);SemaphoreHandle_t done=xSemaphoreCreateBinary();if(!done){free(copy);return ESP_ERR_NO_MEM;}esp_err_t result=ESP_FAIL;Owned item{copy,v.size,v.ingress,done,&result};if(xQueueSend(impl_->queue,&item,0)!=pdTRUE){vSemaphoreDelete(done);free(copy);return ESP_ERR_NO_MEM;}if(xSemaphoreTake(done,pdMS_TO_TICKS(timeout))!=pdTRUE){/* Worker may still own the buffer/result pointer. A timed out fiscal call is an unknown result; keep the waiter alive until the serialized worker resolves to avoid use-after-free. */xSemaphoreTake(done,portMAX_DELAY);result=ESP_ERR_TIMEOUT;}vSemaphoreDelete(done);free(copy);return result;}
esp_err_t queued_command_sink(const CommandView&v,void*context){auto*q=static_cast<CommandQueue*>(context);return q?q->enqueue(v):ESP_ERR_INVALID_STATE;}
}
