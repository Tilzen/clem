Feature: Greeting service
  As a user
  I want a deterministic greeting
  So that agents can verify behaviour via living specs

  Scenario: default greeting
    Given the greeting service is initialized
    When I request a greeting for "world"
    Then the response should be "hello, world"
