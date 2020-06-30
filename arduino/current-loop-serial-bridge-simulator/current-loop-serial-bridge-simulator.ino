void setup()
{
  // Inits the Serial for viewing data in the serial monitor
  Serial.begin(115200);
  delay(100);
}


void loop()
{
  String output = "384|0.42|11.34";
    
  Serial.println(output);

  delay(2000);
}
    
